/*
Copyright 2025 LC.

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
	reclustercomv1alpha1 "github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// RcPolicyLister helps list RcPolicies.
// All objects returned here must be treated as read-only.
type RcPolicyLister interface {
	// List lists all RcPolicies in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*reclustercomv1alpha1.RcPolicy, err error)
	// RcPolicies returns an object that can list and get RcPolicies.
	RcPolicies(namespace string) RcPolicyNamespaceLister
	RcPolicyListerExpansion
}

// rcPolicyLister implements the RcPolicyLister interface.
type rcPolicyLister struct {
	listers.ResourceIndexer[*reclustercomv1alpha1.RcPolicy]
}

// NewRcPolicyLister returns a new RcPolicyLister.
func NewRcPolicyLister(indexer cache.Indexer) RcPolicyLister {
	return &rcPolicyLister{listers.New[*reclustercomv1alpha1.RcPolicy](indexer, reclustercomv1alpha1.Resource("rcpolicy"))}
}

// RcPolicies returns an object that can list and get RcPolicies.
func (s *rcPolicyLister) RcPolicies(namespace string) RcPolicyNamespaceLister {
	return rcPolicyNamespaceLister{listers.NewNamespaced[*reclustercomv1alpha1.RcPolicy](s.ResourceIndexer, namespace)}
}

// RcPolicyNamespaceLister helps list and get RcPolicies.
// All objects returned here must be treated as read-only.
type RcPolicyNamespaceLister interface {
	// List lists all RcPolicies in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*reclustercomv1alpha1.RcPolicy, err error)
	// Get retrieves the RcPolicy from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*reclustercomv1alpha1.RcPolicy, error)
	RcPolicyNamespaceListerExpansion
}

// rcPolicyNamespaceLister implements the RcPolicyNamespaceLister
// interface.
type rcPolicyNamespaceLister struct {
	listers.ResourceIndexer[*reclustercomv1alpha1.RcPolicy]
}
