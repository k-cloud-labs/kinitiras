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
	"github.com/k-cloud-labs/pkg/utils/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// clusterValidatePolicyLister implements the ClusterValidatePolicyLister interface.
type unstructuredClusterValidatePolicyLister struct {
	indexer cache.Indexer
}

// NewUnstructuredClusterValidatePolicyLister returns a new ClusterValidatePolicyLister.
func NewUnstructuredClusterValidatePolicyLister(indexer cache.Indexer) v1alpha1.ClusterValidatePolicyLister {
	return &unstructuredClusterValidatePolicyLister{indexer: indexer}
}

// List lists all ClusterValidatePolicies in the indexer.
func (s *unstructuredClusterValidatePolicyLister) List(selector labels.Selector) (ret []*policyv1alpha1.ClusterValidatePolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		cvp, _ := util.ConvertToClusterValidatePolicy(m.(*unstructured.Unstructured))
		ret = append(ret, cvp)
	})
	return ret, err
}

// Get retrieves the ClusterValidatePolicy from the index for a given name.
func (s *unstructuredClusterValidatePolicyLister) Get(name string) (*policyv1alpha1.ClusterValidatePolicy, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apierrors.NewNotFound(policyv1alpha1.Resource("clustervalidatepolicy"), name)
	}
	cvp, _ := util.ConvertToClusterValidatePolicy(obj.(*unstructured.Unstructured))
	return cvp, nil
}
