// internal/state/types.go
//
// Central “live view” interface for recluster-sync.
//
// * It is **read-only**: callers can list objects but must not mutate them.
// * It embeds controller-runtime’s manager.Runnable, so main.go can simply
//   `mgr.Add(stateObj)` and the cache will be started / stopped with the
//   controller-manager.
// * All accessor methods must be **thread-safe** – they can be invoked from
//   any goroutine once Start(ctx) has returned.

package state

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	reclusterv1alpha1 "github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"
)

// State is a read-only, informer-driven cache of live objects.
type State interface {
	manager.Runnable // <- Start(ctx) will be called by ctrl-manager

	Pods() []*corev1.Pod
	RcNodes() []*reclusterv1alpha1.RcNode
	RcPolicies() []*reclusterv1alpha1.RcPolicy
}
