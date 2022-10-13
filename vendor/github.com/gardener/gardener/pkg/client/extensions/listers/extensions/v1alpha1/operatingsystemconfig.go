/*
Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// OperatingSystemConfigLister helps list OperatingSystemConfigs.
// All objects returned here must be treated as read-only.
type OperatingSystemConfigLister interface {
	// List lists all OperatingSystemConfigs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.OperatingSystemConfig, err error)
	// OperatingSystemConfigs returns an object that can list and get OperatingSystemConfigs.
	OperatingSystemConfigs(namespace string) OperatingSystemConfigNamespaceLister
	OperatingSystemConfigListerExpansion
}

// operatingSystemConfigLister implements the OperatingSystemConfigLister interface.
type operatingSystemConfigLister struct {
	indexer cache.Indexer
}

// NewOperatingSystemConfigLister returns a new OperatingSystemConfigLister.
func NewOperatingSystemConfigLister(indexer cache.Indexer) OperatingSystemConfigLister {
	return &operatingSystemConfigLister{indexer: indexer}
}

// List lists all OperatingSystemConfigs in the indexer.
func (s *operatingSystemConfigLister) List(selector labels.Selector) (ret []*v1alpha1.OperatingSystemConfig, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.OperatingSystemConfig))
	})
	return ret, err
}

// OperatingSystemConfigs returns an object that can list and get OperatingSystemConfigs.
func (s *operatingSystemConfigLister) OperatingSystemConfigs(namespace string) OperatingSystemConfigNamespaceLister {
	return operatingSystemConfigNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// OperatingSystemConfigNamespaceLister helps list and get OperatingSystemConfigs.
// All objects returned here must be treated as read-only.
type OperatingSystemConfigNamespaceLister interface {
	// List lists all OperatingSystemConfigs in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.OperatingSystemConfig, err error)
	// Get retrieves the OperatingSystemConfig from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.OperatingSystemConfig, error)
	OperatingSystemConfigNamespaceListerExpansion
}

// operatingSystemConfigNamespaceLister implements the OperatingSystemConfigNamespaceLister
// interface.
type operatingSystemConfigNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all OperatingSystemConfigs in the indexer for a given namespace.
func (s operatingSystemConfigNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.OperatingSystemConfig, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.OperatingSystemConfig))
	})
	return ret, err
}

// Get retrieves the OperatingSystemConfig from the indexer for a given namespace and name.
func (s operatingSystemConfigNamespaceLister) Get(name string) (*v1alpha1.OperatingSystemConfig, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("operatingsystemconfig"), name)
	}
	return obj.(*v1alpha1.OperatingSystemConfig), nil
}
