// doc.go
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:object:generate=true
// +k8s:deepcopy-gen=package,register
// +groupName=recluster.com

// Package v1alpha1 contains the Recluster v1alpha1 API.
//
// The sole purpose of this file is to let k8s.io/code-generator know
// the <group,version> for this directory so that it can emit the
//
//	SchemeGroupVersion  and  Resource(...)  helpers used by the
//	client / lister / informer code it will generate in the next step.
package v1alpha1
