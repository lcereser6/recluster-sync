// rcpolicy_types.go
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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// +genclient
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// RcPolicy describes *how* the scheduler/extender should rank candidate
// RcNode objects for a Pod that matches `spec.selector`.
//
// What is new in this revision?
//   • **Time‑aware overrides** – `spec.schedule` lets you re‑shape metric
//     weights inside arbitrary time‑windows (e.g. "cheap power after 22:00").
//   • **External feeds** – reference a URL / ConfigMap that exposes live
//     values (spot‑price, carbon intensity…). A CEL expression converts
//     that value into a weight multiplier *per metric*.
//
// The core idea is still intentionally simple: the runtime scorer takes the
// *base* `spec.metrics`, applies
//    1. the first matching `schedule` window (if any); then
//    2. all `externalFeeds` transformations (if any)
// and finally produces the familiar weighted‑sum or lexicographic ordering.
//
// Nothing in the existing RcPolicy YAMLs breaks – you only add two optional
// fields.

type RcPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RcPolicySpec   `json:"spec"`
	Status RcPolicyStatus `json:"status,omitempty"`
}

/* -------------------------------------------------------------------------- */
/*                                   Spec                                     */
/* -------------------------------------------------------------------------- */

type RcPolicySpec struct {
	// selector matches Pods that *use* this policy.
	// If nil, the policy is considered the cluster‑default.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// List of metrics that feed the scoring function.
	Metrics []PolicyMetric `json:"metrics"`

	// Hard constraints – CEL expressions evaluated per candidate assignment.
	// If any evaluates to *false* the node is rejected.
	// +optional
	HardConstraints []PolicyConstraint `json:"hardConstraints,omitempty"`

	// Optional time‑based overrides.
	// The first entry whose window contains *now()* overrides the base metric
	// definitions (weight/multiplier). Think of them as “profiles”.
	// +optional
	Schedule []PolicyScheduleEntry `json:"schedule,omitempty"`

	// Optional external inputs (spot‑price feeds, carbon intensity APIs…).
	// Each feed is fetched by the controller; its numeric output is passed to
	// a CEL transform that yields per‑metric multipliers.
	// +optional
	ExternalFeeds []ExternalFeedRef `json:"externalFeeds,omitempty"`
}

/* --------------------------- Metrics & helpers ---------------------------- */

// ValueFrom declares where a metric is read from inside RcNode.
// • jsonPath  – default, evaluated against the *whole* RcNode object.
// • fieldPath – uses the downward‑API syntax (metadata.labels['x'] …).
//
// Additional sources (Prometheus, metrics‑server…) could be added later.

type ValueFrom string

const (
	ValueFromJSONPath  ValueFrom = "jsonPath"
	ValueFromFieldPath ValueFrom = "fieldPath"
)

type PolicyMetric struct {
	// Key is just a symbolic handle used by policies/schedule/feed mappings.
	Key string `json:"key"`

	// Weight – if you stick to the weighted‑sum model, positive means
	// “*minimise* this metric”, negative means “*maximise*”.
	// If you switch to lexicographic ordering the runtime can ignore it.
	Weight float64 `json:"weight"`

	// Source + Selector tell the runtime *where* to fetch the value.
	// Example (jsonPath):   $.status.predictedPowerWatts
	// Example (fieldPath):  metadata.labels['topology.kubernetes.io/zone']
	// +optional (defaults to jsonPath + Key)
	Source   ValueFrom `json:"source,omitempty"`
	Selector string    `json:"selector,omitempty"`

	// Optional CEL transform executed *after* the value is fetched and before
	// weighting. Use this for unit conversions or capping.
	// e.g. "min(x, 180)" where `x` is the fetched value.
	// +optional
	Transform *string `json:"transform,omitempty"`
}

// PolicyConstraint is unchanged: a CEL boolean that must hold true.

type PolicyConstraint struct {
	Expression string `json:"expression"`
}

/* --------------------------- Time‑based overrides ------------------------- */

// PolicyScheduleEntry replaces/adjusts metric weights in a given window.
//
// Time windows are evaluated *in cluster‑local time* (whatever the controller
// runs with). They repeat every 24 hours. More elaborate RRULE support can
// be added later without changing existing YAML.

type PolicyScheduleEntry struct {
	// Name is only for human debugging.
	Name string `json:"name"`

	// Start & End – 24‑hour clock in "HH:MM" (e.g. "22:00").
	// The window is inclusive of Start and exclusive of End.
	Start string `json:"start"`
	End   string `json:"end"`

	// Adjustments: either *replace* the weight or *multiply* it.
	// If both are set, Replace takes precedence.
	Adjustments []MetricAdjustment `json:"adjustments"`
}

// MetricAdjustment targets one metric by `key`.
// Exactly one of Replace / Multiply should be supplied.

type MetricAdjustment struct {
	Key      string   `json:"key"`
	Replace  *float64 `json:"replace,omitempty"`
	Multiply *float64 `json:"multiply,omitempty"`
}

/* ------------------------------ External feeds --------------------------- */

// ExternalFeedRef lets a policy depend on outside data (spot electricity
// price, CO₂ intensity, datacentre cooling PUE …).
//
// A separate controller fetches the URL (JSON/Prometheus/gRPC etc.) and
// stores the **latest numeric value** in `.status.feeds[name].value` so the
// scorer can be fully offline.

type ExternalFeedRef struct {
	Name string `json:"name"` // unique identifier inside the policy
	URL  string `json:"url"`  // pull endpoint (or k8s://cm/secret/... later)

	// How each metric reacts to the feed value – a CEL expression that receives
	// the variable `$value` (float64) and outputs the multiplier.
	// e.g.  multiplier = 1 + ($value / 100)   // raise cost when price high
	Mappings []FeedMetricMapping `json:"mappings"`
}

type FeedMetricMapping struct {
	Key       string `json:"key"`       // metric to alter
	Transform string `json:"transform"` // CEL producing the multiplier
}

/* ------------------------------ Status ----------------------------------- */

type RcPolicyStatus struct {
	MatchedPods  int32       `json:"matchedPods,omitempty"`
	RejectedPods int32       `json:"rejectedPods,omitempty"`
	LastResolved metav1.Time `json:"lastResolved,omitempty"`
	// Last time an external feed was updated.
	LastFeedSync *metav1.Time `json:"lastFeedSync,omitempty"`
}

/* ------------------------------ Runtime helpers -------------------------- */

// CompiledSelector caches the label.Selector (nil → match‑all).
func (p *RcPolicy) CompiledSelector() (labels.Selector, error) {
	if p.Spec.Selector == nil {
		return labels.Everything(), nil
	}
	return metav1.LabelSelectorAsSelector(p.Spec.Selector)
}

// ActiveSchedule returns the first schedule window that contains *now*.
// Returns (nil, false) if no window matches.
func (p *RcPolicy) ActiveSchedule(now time.Time) (*PolicyScheduleEntry, bool) {
	local := now
	for _, s := range p.Spec.Schedule {
		start, err := time.Parse("15:04", s.Start)
		if err != nil {
			continue // ignore malformed windows
		}
		end, err := time.Parse("15:04", s.End)
		if err != nil {
			continue
		}
		// Build today’s window in local clock
		st := time.Date(local.Year(), local.Month(), local.Day(), start.Hour(), start.Minute(), 0, 0, local.Location())
		ed := time.Date(local.Year(), local.Month(), local.Day(), end.Hour(), end.Minute(), 0, 0, local.Location())
		if !ed.After(st) { // overnight window (e.g. 22:00‑06:00)
			ed = ed.Add(24 * time.Hour)
			if local.Before(st) {
				local = local.Add(24 * time.Hour) // normalise
			}
		}
		if (local.Equal(st) || local.After(st)) && local.Before(ed) {
			return &s, true
		}
	}
	return nil, false
}

/* ------------------------------ List type -------------------------------- */

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RcPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RcPolicy `json:"items"`
}

/* ------------------------------ Registration ----------------------------- */

func init() {
	SchemeBuilder.Register(&RcPolicy{}, &RcPolicyList{})
}
