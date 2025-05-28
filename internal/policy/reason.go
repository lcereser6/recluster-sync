package policy

// ResolutionReason explains how a pod/policy decision was made.
// Handy for metrics & events.
type ResolutionReason string

const (
	ReasonSkipAnnotation   ResolutionReason = "skip-annotation"
	ReasonExactName        ResolutionReason = "exact-name"
	ReasonSelectorMatch    ResolutionReason = "selector-match"
	ReasonDefaultPolicy    ResolutionReason = "cluster-default"
	ReasonNoneFound        ResolutionReason = "no-policy-found"
	ReasonNameNotFound     ResolutionReason = "name-not-found"
	ReasonConflictingHints ResolutionReason = "conflicting-annotations"
)
