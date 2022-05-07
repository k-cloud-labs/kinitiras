/*
Copyright 2022 by k-cloud-labs org.

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

package lister

import (
	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/util/converter"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
)

// unstructuredOverridePolicyLister implements the OverridePolicyLister interface.
type unstructuredOverridePolicyLister struct {
	indexer cache.Indexer
}

// NewOverridePolicyLister returns a new OverridePolicyLister.
func NewUnstructuredOverridePolicyLister(indexer cache.Indexer) v1alpha1.OverridePolicyLister {
	return &unstructuredOverridePolicyLister{indexer: indexer}
}

// List lists all OverridePolicies in the indexer.
func (s *unstructuredOverridePolicyLister) List(selector labels.Selector) (ret []*policyv1alpha1.OverridePolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		op, _ := converter.ConvertToOverridePolicy(m.(*unstructured.Unstructured))
		ret = append(ret, op)
	})
	return ret, err
}

// OverridePolicies returns an object that can list and get OverridePolicies.
func (s *unstructuredOverridePolicyLister) OverridePolicies(namespace string) v1alpha1.OverridePolicyNamespaceLister {
	return unstructuredOverridePolicyNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// unstructuredOverridePolicyNamespaceLister implements the OverridePolicyNamespaceLister
// interface.
type unstructuredOverridePolicyNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all OverridePolicies in the indexer for a given namespace.
func (s unstructuredOverridePolicyNamespaceLister) List(selector labels.Selector) (ret []*policyv1alpha1.OverridePolicy, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		op, _ := converter.ConvertToOverridePolicy(m.(*unstructured.Unstructured))
		ret = append(ret, op)
	})
	return ret, err
}

// Get retrieves the OverridePolicy from the indexer for a given namespace and name.
func (s unstructuredOverridePolicyNamespaceLister) Get(name string) (*policyv1alpha1.OverridePolicy, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apierrors.NewNotFound(policyv1alpha1.Resource("overridepolicy"), name)
	}
	op, _ := converter.ConvertToOverridePolicy(obj.(*unstructured.Unstructured))
	return op, nil
}
