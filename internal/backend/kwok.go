package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	rcv1 "github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	typedcore "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
)

const (
	kwokManagedAnnotation = "kwok.x-k8s.io/node" // value = "fake"
)

// ----------------------------------------------------------------------------
// Constructor
// ----------------------------------------------------------------------------
type kwokBackend struct {
	core typedcore.CoreV1Interface
}

func NewKwokBackend(k8s kubernetes.Interface) *kwokBackend {
	return &kwokBackend{core: k8s.CoreV1()}
}

// ----------------------------------------------------------------------------
// Reconcile
// ----------------------------------------------------------------------------
func (b *kwokBackend) Reconcile(ctx context.Context, rc *rcv1.RcNode) error {
	wantRunning := rc.Spec.DesiredState == "Running"
	nodeName := templateNodeName(rc) // "kwok-fake-<rcname>"
	providerID := fmt.Sprintf("recluster://%s", rc.Name)

	klog.Infof("KWOK: reconcile RcNode %q (%s) -> %q", rc.Name, rc.Spec.DesiredState, nodeName)
	// does the Node already exist?
	node, err := b.core.Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil && !isNotFound(err) {
		return err
	}

	switch {
	// ----------------------------------------------------------------------
	// 1) RcNode says it should run  ►  ensure Node exists & Ready=True
	// ----------------------------------------------------------------------
	case wantRunning:
		if isNotFound(err) {
			// create brand-new node
			klog.Infof("KWOK: creating fake node %q for RcNode %q", nodeName, rc.Name)
			_, err = b.core.Nodes().Create(ctx, buildKwokNode(rc, nodeName, providerID),
				metav1.CreateOptions{})
			return err
		}

		// exists – patch ProviderID + Ready=True if needed
		return b.ensureReady(ctx, node, providerID)

	// ----------------------------------------------------------------------
	// 2) RcNode wants it stopped  ►  delete Node if present
	// ----------------------------------------------------------------------
	default:
		if isNotFound(err) {
			return nil // nothing to do
		}
		klog.Infof("KWOK: deleting fake node %q (RcNode %q wants Stopped)", nodeName, rc.Name)
		return b.core.Nodes().Delete(ctx, nodeName, metav1.DeleteOptions{})
	}
}

// ----------------------------------------------------------------------------
// helpers
// ----------------------------------------------------------------------------
func buildKwokNode(rc *rcv1.RcNode, nodeName, providerID string) *corev1.Node {
	cpuMillicores := rc.Spec.CPU.Cores * 1000
	memBytes := int64(rc.Spec.Memory) * 1024 * 1024 * 1024
	klog.Infof("KWOK: creating fake node (RcNode %q wants %d cores, %d GiB)", rc.Name, rc.Spec.CPU.Cores, rc.Spec.Memory)
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				"kubernetes.io/cores": fmt.Sprintf("%d", rc.Spec.CPU.Cores), // "amd64" or "arm64"
				"kubernetes.io/os":    "linux",
				"kubernetes.io/arch":  "amd64",
			},
			Annotations: map[string]string{
				kwokManagedAnnotation: "fake",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID: providerID,
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewMilliQuantity(int64(cpuMillicores), resource.DecimalSI),
				corev1.ResourceMemory: *resource.NewQuantity(memBytes, resource.BinarySI),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewMilliQuantity(int64(cpuMillicores), resource.DecimalSI),
				corev1.ResourceMemory: *resource.NewQuantity(memBytes, resource.BinarySI),
			},
			Conditions: []corev1.NodeCondition{readyCond(corev1.ConditionTrue)},
		},
	}
}

func readyCond(status corev1.ConditionStatus) corev1.NodeCondition {
	return corev1.NodeCondition{
		Type:               corev1.NodeReady,
		Status:             status,
		LastHeartbeatTime:  metav1.Time{Time: time.Now()},
		LastTransitionTime: metav1.Time{Time: time.Now()},
		Reason:             "KwokReady",
		Message:            "kwok instant ready",
	}
}

func (b *kwokBackend) ensureReady(ctx context.Context, node *corev1.Node, providerID string) error {
	needPatch := false
	after := node.DeepCopy()

	if after.Spec.ProviderID != providerID {
		after.Spec.ProviderID = providerID
		needPatch = true
	}
	if cond := getReadyCond(after); cond == nil || cond.Status != corev1.ConditionTrue {
		after.Status.Conditions = mergeReady(after.Status.Conditions, readyCond(corev1.ConditionTrue))
		needPatch = true
	}

	if !needPatch {
		return nil
	}
	patch, _ := strategicpatch.CreateTwoWayMergePatch(
		[]byte(mustJSON(node)), []byte(mustJSON(after)), corev1.Node{})
	_, err := b.core.Nodes().Patch(ctx, node.Name, types.StrategicMergePatchType, patch,
		metav1.PatchOptions{}, "status")
	return err
}

func getReadyCond(n *corev1.Node) *corev1.NodeCondition {
	for i := range n.Status.Conditions {
		if n.Status.Conditions[i].Type == corev1.NodeReady {
			return &n.Status.Conditions[i]
		}
	}
	return nil
}

func mergeReady(list []corev1.NodeCondition, ready corev1.NodeCondition) []corev1.NodeCondition {
	out := list
	for i := range out {
		if out[i].Type == corev1.NodeReady {
			out[i] = ready
			return out
		}
	}
	return append(out, ready)
}

func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

// templateNodeName replicates the naming used in the plan
func templateNodeName(rc *rcv1.RcNode) string {
	return fmt.Sprintf("kwok-fake-%s", rc.Name)
}

// crude helper – marshal without error handling for patch creation
func mustJSON(obj interface{}) string {
	data, _ := json.Marshal(obj)
	return string(data)
}
