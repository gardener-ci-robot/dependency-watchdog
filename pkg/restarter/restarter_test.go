// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package restarter

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	test "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/testutil"
)

var (
	dep = `
namespace: default
services:
  kube-apiserver:
    dependantPods:
    - name: controlplane
      selector:
        matchExpressions:
        - key: garden.sapcloud.io/role
          operator: In
          values:
          - controlplane`
	//        - key: role
	//          operator: NotIn
	//          values:
	//          - main`

	watchDuration = 2 * 60 * time.Second
	alwaysReady   = func() bool { return true }
	neverReady    = func() bool { return false }
)

type fixture struct {
	t *testing.T
	// Actions expected to happen on the client. Objects from here are also
	// preloaded into NewSimpleFake.
	objects []runtime.Object
	client  kubeclient.Interface
	// Objects to put in the store.
	endpoints []*v1.Endpoints
}

type depController struct {
	*Controller
	podStore       cache.Store
	endpointsStore cache.Store
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	return f
}

func newEndpoint(name, namespace string, labels map[string]string) *v1.Endpoints {
	nodeName := "docker-for-desktop"
	e := v1.Endpoints{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Endpoints"},
		ObjectMeta: metav1.ObjectMeta{
			UID:         uuid.NewUUID(),
			Name:        name,
			Namespace:   namespace,
			Annotations: make(map[string]string),
			Labels:      labels,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						IP:       "10.1.0.52",
						NodeName: &nodeName,
					},
				},
				NotReadyAddresses: []v1.EndpointAddress{},
				Ports:             []v1.EndpointPort{},
			},
		},
	}
	return &e
}

func newPodInCrashloop(name string, labels map[string]string) *v1.Pod {
	p := testutil.NewPod(name, "node-0")
	p.Labels = labels
	p.Namespace = metav1.NamespaceDefault
	p.Status.ContainerStatuses = []v1.ContainerStatus{
		{
			Name: "Container-0",
			State: v1.ContainerState{
				Waiting: &v1.ContainerStateWaiting{
					Reason:  "CrashLoopBackOff",
					Message: "Container is in CrashLoopBackOff.",
				},
			},
		},
	}
	return p
}

func newPodHealthy(name string, labels map[string]string) *v1.Pod {
	p := testutil.NewPod(name, "node-0")
	p.Labels = labels
	p.Namespace = metav1.NamespaceDefault
	p.Status.ContainerStatuses = []v1.ContainerStatus{
		{
			Name: "Container-0",
		},
	}
	return p
}

func makePodUnhealthy(p *v1.Pod) *v1.Pod {
	p.Status.ContainerStatuses = []v1.ContainerStatus{
		{
			Name: "Container-0",
			State: v1.ContainerState{
				Waiting: &v1.ContainerStateWaiting{
					Reason:  "CrashLoopBackOff",
					Message: "Container is in CrashLoopBackOff.",
				},
			},
		},
	}
	return p
}

func (f *fixture) newController(deps *ServiceDependants, stopCh chan struct{}) (*Controller, informers.SharedInformerFactory, error) {

	informers := informers.NewSharedInformerFactoryWithOptions(
		f.client,
		controller.NoResyncPeriodFunc(),
		informers.WithNamespace(deps.Namespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {}))

	c := NewController(f.client, informers, deps, watchDuration, stopCh)
	for _, d := range f.endpoints {
		informers.Apps().V1().Deployments().Informer().GetIndexer().Add(d)
	}
	return c, informers, nil
}

func TestDeleteOnlyCrashloopBackoffPods(t *testing.T) {
	f := newFixture(t)
	deps, err := DecodeConfigFile([]byte(dep))
	if err != nil {
		t.Fatalf("error decoding file: %v", err)
	}
	deps.Namespace = metav1.NamespaceDefault
	stopCh := make(chan struct{})
	defer close(stopCh)

	const (
		healthyPod  = "pod-h"
		crashingPod = "pod-c"
	)

	depMap, err := metav1.LabelSelectorAsMap(deps.Services["kube-apiserver"].Dependants[0].Selector)
	if err != nil {
		t.Fatalf("error creating map from selector: %v", err)
	}
	e := newEndpoint("kube-apiserver", deps.Namespace, depMap)
	pC := newPodInCrashloop(crashingPod, map[string]string{
		"garden.sapcloud.io/role": "controlplane",
		"role":                    "NotEtcd",
	})
	pH := newPodHealthy(healthyPod, map[string]string{
		"garden.sapcloud.io/role": "controlplane",
		"role":                    "NotEtcd",
	})

	f.endpoints = append(f.endpoints, e)
	f.objects = append(f.objects, e, pC, pH)
	watcher := watch.NewFakeWithChanSize(2, false)
	client := fake.NewSimpleClientset(f.objects...)
	client.PrependWatchReactor("pods", test.DefaultWatchReactor(watcher, nil))
	f.client = client
	// simulate add/update/delete watch events

	c, _, err := f.newController(deps, stopCh)
	if err != nil {
		t.Fatalf("error creating Deployment controller: %v", err)
	}

	watcher.Add(pC)
	watcher.Add(pH)

	pl, err := f.client.CoreV1().Pods(metav1.NamespaceDefault).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("error fetching pods: %v", err)
	}
	if len(pl.Items) != 2 {
		t.Errorf("Error setting up the test case. Expected 2 pods but got %d", len(pl.Items))
	}

	go func() {
		t.Logf("Starting dep watchdog.\n")
		c.Run(1)
	}()

	// Wait for the dependency watchdog to take action.
	time.Sleep(2 * time.Second)

	pl, err = f.client.CoreV1().Pods(metav1.NamespaceDefault).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("error fetching pods: %v", err)
	}

	if len(pl.Items) != 1 {
		t.Errorf("Pod in CrashloopBackoff not deleted by the dependency-watchdog. Expected 1 pods but got %d", len(pl.Items))
	}

	if pl.Items[0].Name != healthyPod {
		t.Errorf("Pod in CrashloopBackoff not deleted by the dependency-watchdog. Expected the remaining pod to be %s but was %s", healthyPod, pl.Items[0].Name)
	}

}

func TestDeletePodTransitioningToCrashloopBackoff(t *testing.T) {
	f := newFixture(t)
	deps, err := DecodeConfigFile([]byte(dep))
	if err != nil {
		t.Fatalf("error decoding file: %v", err)
	}
	deps.Namespace = metav1.NamespaceDefault
	stopCh := make(chan struct{})
	defer close(stopCh)

	const (
		healthyPod = "pod-h"
	)

	depMap, err := metav1.LabelSelectorAsMap(deps.Services["kube-apiserver"].Dependants[0].Selector)
	if err != nil {
		t.Fatalf("error creating map from selector: %v", err)
	}
	e := newEndpoint("kube-apiserver", deps.Namespace, depMap)
	// pC := newPodInCrashloop("pod-0", map[string]string{
	// 	"garden.sapcloud.io/role": "controlplane",
	// 	"role":                    "NotEtcd",
	// })
	pH := newPodHealthy(healthyPod, map[string]string{
		"garden.sapcloud.io/role": "controlplane",
		"role":                    "NotEtcd",
	})

	f.endpoints = append(f.endpoints, e)
	f.objects = append(f.objects, e, pH)
	watcher := watch.NewFakeWithChanSize(1, false)
	client := fake.NewSimpleClientset(f.objects...)
	client.PrependWatchReactor("pods", test.DefaultWatchReactor(watcher, nil))
	f.client = client
	// simulate add/update/delete watch events

	c, _, err := f.newController(deps, stopCh)
	if err != nil {
		t.Fatalf("error creating Deployment controller: %v", err)
	}

	pl, err := f.client.CoreV1().Pods(metav1.NamespaceDefault).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("error fetching pods: %v", err)
	}
	watcher.Add(pH)
	pl, err = f.client.CoreV1().Pods(metav1.NamespaceDefault).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("error fetching pods: %v", err)
	}

	if len(pl.Items) != 1 {
		t.Errorf("Error setting up the test case. Expected 1 pod but got %d", len(pl.Items))
	}

	go func() {
		t.Logf("Starting dep watchdog.\n")
		c.Run(1)
	}()

	t.Logf("Making pod go into CrashloopBackoff and wait for 2 seconds.")
	pU, err := f.client.CoreV1().Pods(metav1.NamespaceDefault).Update(makePodUnhealthy(pH))
	if err != nil {
		t.Fatalf("error updating pods: %v", err)
	}
	watcher.Modify(pU)
	time.Sleep(2 * time.Second)

	pl, err = f.client.CoreV1().Pods(metav1.NamespaceDefault).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("error fetching pods: %v", err)
	}
	if len(pl.Items) != 0 {
		t.Errorf("Pod in CrashloopBackoff not deleted by the dependency-watchdog. Expected 0 pods but got %d", len(pl.Items))
	}
}
