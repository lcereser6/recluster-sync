// graph/step.go
package graph

// const gateKey = "recluster-sync/waiting-for-node"

// func RunStep(now time.Time,
// 	pods []*corev1.Pod,
// 	rcnodes []*reclusterv1.RcNode,
// 	policies []*reclusterv1.RcPolicy,
// 	minIdleSeconds int) []Action {

// 	// ---------------------------------------------------------------------
// 	// 1. Which Pods still need placement?
// 	// ---------------------------------------------------------------------
// 	var pending []*corev1.Pod
// 	for _, p := range pods {
// 		if hasGate(p) && p.Annotations["recluster.io/rcnode"] == "" {
// 			pending = append(pending, p)
// 		}
// 	}
// 	klog.Infof("RunStep: pods=%d pending=%d nodes=%d policies=%d",
// 		len(pods), len(pending), len(rcnodes), len(policies))

// 	// ---------------------------------------------------------------------
// 	// 2. Node bookkeeping: who is already running / booting / idle-long?
// 	// ---------------------------------------------------------------------
// 	nodeNeeded := map[string]bool{} // any Pod still needs this node
// 	nodeBootETA := map[string]time.Time{}
// 	for _, n := range rcnodes {
// 		if n.Status.State == reclusterv1.NodeStatusBooting {
// 			eta := n.Status.LastTransition.Time.
// 				Add(time.Duration(n.Spec.BootSeconds) * time.Second)
// 			nodeBootETA[n.Name] = eta
// 		}
// 	}

// 	// ---------------------------------------------------------------------
// 	// 3. Solve per-pod placement
// 	// ---------------------------------------------------------------------
// 	var acts []Action
// 	for _, pod := range pending {
// 		pol := resolvePolicy(pod, policies)
// 		if pol == nil {
// 			klog.Warningf("no policy matched pod %s/%s", pod.Namespace, pod.Name)
// 			continue
// 		}

// 		best := solver.PickBest(rcnodes, pol)
// 		if best == nil {
// 			klog.Infof("no node fits pod %s/%s", pod.Namespace, pod.Name)
// 			continue
// 		}
// 		nodeNeeded[best.Name] = true

// 		acts = append(acts, PodPatch{
// 			Pod: *pod,
// 			Annotations: map[string]string{
// 				"recluster.io/rcnode": best.Name,
// 			},
// 			RemoveGate: true,
// 			Tolerations: []corev1.Toleration{ // runtime-generated toleration
// 				{
// 					Key:      "recluster.io/node",
// 					Operator: corev1.TolerationOpEqual,
// 					Value:    best.Name,
// 					Effect:   corev1.TaintEffectNoSchedule,
// 				},
// 			},
// 		})
// 	}

// 	// ---------------------------------------------------------------------
// 	// 4. Decide node start / stop
// 	// ---------------------------------------------------------------------
// 	for _, n := range rcnodes {
// 		switch {
// 		case nodeNeeded[n.Name] && n.Status.State != reclusterv1.NodeStatusRunning:
// 			acts = append(acts, NodeAction{
// 				Node:    *n,
// 				Kind:    NodeStart,
// 				ReadyAt: now.Add(time.Duration(n.Spec.BootSeconds) * time.Second),
// 				Reason:  "pod waiting",
// 			})
// 		case !nodeNeeded[n.Name] &&
// 			n.Status.State == reclusterv1.NodeStatusRunning &&
// 			now.Sub(n.Status.LastTransition.Time) >= time.Duration(minIdleSeconds)*time.Second:
// 			acts = append(acts, NodeAction{
// 				Node:    *n,
// 				Kind:    NodeStop,
// 				ReadyAt: now,
// 				Reason:  "idle timeout",
// 			})
// 		default:
// 			// keep alive or still booting
// 		}
// 	}
// 	return dedupNodeActions(acts)
// }
