//rcpolicy_types.go

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// +genclient

// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RcPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RcPolicySpec   `json:"spec"`
	Status RcPolicyStatus `json:"status,omitempty"`
}

// ---------------------------------------------------------------------------
// Spec
// ---------------------------------------------------------------------------

type RcPolicySpec struct {
	// Selector matches Pods that should use this policy.
	// Omit to designate this as the *cluster-default* policy.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Metrics & hard-constraints will be consumed by the solver in Task 2.
	Metrics         []PolicyMetric     `json:"metrics"`
	HardConstraints []PolicyConstraint `json:"hardConstraints,omitempty"`
}

type PolicyMetric struct {
	// Data-point name exposed by RcNode / telemetry.
	Key string `json:"key"`

	// Weight (positive = minimise, negative = maximise).
	Weight float64 `json:"weight"`

	// Optional CEL transform (Task 2).
	// +optional
	Transform *string `json:"transform,omitempty"`
}

type PolicyConstraint struct {
	// CEL expression that must evaluate to true on a candidafalte assignment.
	Expression string `json:"expression"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RcPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RcPolicy `json:"items"`
}

type RcPolicyStatus struct {
	MatchedPods  int32       `json:"matchedPods,omitempty"`
	RejectedPods int32       `json:"rejectedPods,omitempty"`
	LastResolved metav1.Time `json:"lastResolved,omitempty"`
}

// ---------------------------------------------------------------------------
// Helpers (non-generated)
// ---------------------------------------------------------------------------

// CompiledSelector returns a cached labels.Selector (nil means "match all").
func (p *RcPolicy) CompiledSelector() (labels.Selector, error) {
	if p.Spec.Selector == nil {
		return labels.Everything(), nil
	}
	return metav1.LabelSelectorAsSelector(p.Spec.Selector)
}

func init() {
	SchemeBuilder.Register(&RcPolicy{}, &RcPolicyList{})
}
