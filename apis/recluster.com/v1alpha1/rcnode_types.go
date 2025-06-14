// rcnode_types.go
/*
Copyright 2025.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Rcnode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RcnodeSpec   `json:"spec,omitempty"`
	Status RcnodeStatus `json:"status,omitempty"`
}

// RcnodeList contains a list of Rcnode.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RcnodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Rcnode `json:"items"`
}

type RcnodeSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	CPUCores     int    `json:"cpuCores"`
	MemoryGiB    int    `json:"memoryGiB"`
	BootSeconds  int    `json:"bootSeconds,omitempty"`  // 0 = powered off, >0 = booting
	DesiredState string `json:"desiredState,omitempty"` // "", "Running", "Stopped"
	Foo          string `json:"foo,omitempty"`
}

type RcnodeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	State string `json:"state,omitempty"` // "", "Running", "Stopped"
}

func init() {
	SchemeBuilder.Register(&Rcnode{}, &RcnodeList{})
}
