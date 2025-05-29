package policy

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	rcv1 "github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"
)

// Annotation / label keys (single source of truth)
const (
	KeyPolicyName  = "recluster.io/policy-name"  // annotation
	KeyPolicyClass = "recluster.io/policy-class" // label
	KeyPolicySkip  = "recluster.io/policy-skip"  // annotation == "true"
)

// ResolveForPod implements the precedence matrix:
//
//  1. skip?            -> (nil, ReasonSkipAnnotation)
//  2. exact name?      -> that policy or error
//  3. selector match   -> first in .Items order
//  4. default policy   -> .selector == nil
//  5. nothing          -> error
func ResolveForPod(
	pod *corev1.Pod,
	policies []rcv1.RcPolicy,
) (*rcv1.RcPolicy, ResolutionReason, error) {

	ann := pod.GetAnnotations()
	lbl := pod.GetLabels()

	// ---------------------------------------------------------------------
	// 1) skip completely?
	// ---------------------------------------------------------------------
	if val, ok := ann[KeyPolicySkip]; ok && val == "true" {
		return nil, ReasonSkipAnnotation, nil
	}

	// Prepare quick lookup maps
	byName := map[string]*rcv1.RcPolicy{}
	var defaults []*rcv1.RcPolicy
	for i := range policies {
		p := &policies[i]
		byName[p.Name] = p
		if p.Spec.Selector == nil {
			defaults = append(defaults, p)
		}
	}

	// ---------------------------------------------------------------------
	// 2) explicit name override?
	// ---------------------------------------------------------------------
	if name, ok := ann[KeyPolicyName]; ok {
		if pol, ok := byName[name]; ok {
			return pol, ReasonExactName, nil
		}
		return nil, ReasonNameNotFound,
			fmt.Errorf("pod requests RcPolicy %q but it does not exist", name)
	}

	// ---------------------------------------------------------------------
	// 3) label/selector match (policy class + arbitrary labels)
	// ---------------------------------------------------------------------
	// Build a selector for the *pod* once so we don't re-build for each policy.
	var podSet labels.Set = lbl

	for i := range policies {
		pol := &policies[i]
		sel, err := pol.CompiledSelector()
		if err != nil {
			return nil, "", err // malformed CRD – let controller surface error
		}
		if sel.Matches(podSet) {
			return pol, ReasonSelectorMatch, nil
		}
	}

	// ---------------------------------------------------------------------
	// 4) fall back to cluster default (deterministically pick oldest)
	// ---------------------------------------------------------------------
	if len(defaults) > 0 {
		sort.Slice(defaults, func(i, j int) bool {
			return defaults[i].CreationTimestamp.Before(&defaults[j].CreationTimestamp)
		})
		return defaults[0], ReasonDefaultPolicy, nil
	}

	// ---------------------------------------------------------------------
	// 5) nothing -> controller will mark pod Unschedulable
	// ---------------------------------------------------------------------
	return nil, ReasonNoneFound, fmt.Errorf("no RcPolicy matches pod %s/%s", pod.Namespace, pod.Name)
}
