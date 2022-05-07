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
	"github.com/k-cloud-labs/pkg/client/listers/policy/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/k-cloud-labs/pkg/util/converter"
)

// clusterOverridePolicyLister implements the ClusterOverridePolicyLister interface.
type unstructuredClusterOverridePolicyLister struct {
	indexer cache.Indexer
}

// NewUnstructuredClusterOverridePolicyLister returns a new ClusterOverridePolicyLister.
func NewUnstructuredClusterOverridePolicyLister(indexer cache.Indexer) v1alpha1.ClusterOverridePolicyLister {
	return &unstructuredClusterOverridePolicyLister{indexer: indexer}
}

// List lists all ClusterOverridePolicies in the indexer.
func (s *unstructuredClusterOverridePolicyLister) List(selector labels.Selector) (ret []*policyv1alpha1.ClusterOverridePolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		cop, _ := converter.ConvertToClusterOverridePolicy(m.(*unstructured.Unstructured))
		ret = append(ret, cop)
	})
	return ret, err
}

// Get retrieves the ClusterOverridePolicy from the index for a given name.
func (s *unstructuredClusterOverridePolicyLister) Get(name string) (*policyv1alpha1.ClusterOverridePolicy, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apierrors.NewNotFound(policyv1alpha1.Resource("clusteroverridepolicy"), name)
	}
	cop, _ := converter.ConvertToClusterOverridePolicy(obj.(*unstructured.Unstructured))
	return cop, nil
}
