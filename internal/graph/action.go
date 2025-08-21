// graph/action.go
package graph

import (
	"time"

	reclusterv1 "github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// Action is a marker interface; every concrete type has a unique Go struct.
// The planner’s upper layer type-switches on these.
type Action interface{ isAction() }

// ---------------------------------------------------------------------------
// Node actions
// ---------------------------------------------------------------------------

type NodeActionKind string

const (
	NodeStart NodeActionKind = "Start"
	NodeStop  NodeActionKind = "Stop"
	NodeNOP   NodeActionKind = "NOP" // already in desired state
)

type NodeAction struct {
	Node reclusterv1.RcNode
	Kind NodeActionKind
	// When the action *should* be considered executed (for cooldowns / races).
	// • Start:  node.Status.LastTransition + BootSeconds
	// • Stop :  immediate, but you can push it to “now + drainTimeout”
	ReadyAt time.Time
	Reason  string
}

func (NodeAction) isAction() {}

// ---------------------------------------------------------------------------
// Pod actions
// ---------------------------------------------------------------------------

type PodPatch struct {
	Pod         corev1.Pod
	Annotations map[string]string
	Tolerations []corev1.Toleration
	RemoveGate  bool // true => delete the scheduling-gate
}

func (PodPatch) isAction() {}
