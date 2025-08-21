// graph/decision.go – action‑builder layer (refactored)
// -----------------------------------------------------------------------------
// This module works purely in‑memory.  From cached cluster objects it produces a
// **flat slice of actions** that the upper‑level planner will later translate
// into Kubernetes API calls (patches, power‑ops via BMC, etc.).
//
//  Action taxonomy
//  ───────────────
//  • NodePowerAction  – power‑on / power‑off decisions (Running ↔ Stopped)
//  • PodScheduleAction – annotate Pod, add tolerations / nodeSelector, remove
//                        our SchedulingGate so the default scheduler binds it
//
//  All future actions can embed the marker interface `Action` to remain
//  polymorphic.
// -----------------------------------------------------------------------------

package graph

/* -------------------------------------------------------------------------- */
/*                               Action types                                 */
/* -------------------------------------------------------------------------- */

// NodePowerAction – request a state transition (Running / Stopped).

// type NodePowerAction struct {
// 	NodeName     string
// 	DesiredState reclusterv1.RcNodeState // Running | Stopped
// 	Reason       string                  // for logging / events
// }

// func (NodePowerAction) isAction() {}

// // PodScheduleAction – patch pod so it can be bound only to TargetNode.
// //   • Adds nodeSelector + toleration (key: recluster.io/node=<name>)
// //   • Removes our SchedulingGate and sets annotation recluster.io/rcnode

// type PodScheduleAction struct {
// 	PodRef       types.NamespacedName
// 	TargetNode   string
// 	Annotations  map[string]string
// 	Tolerations  []corev1.Toleration
// 	NodeSelector map[string]string
// }

// func (PodScheduleAction) isAction() {}

// /* -------------------------------------------------------------------------- */
// /*                                Constants                                   */
// /* -------------------------------------------------------------------------- */

// const (
// 	gateKey               = "recluster-sync/waiting-for-node"
// 	annAssignment         = "recluster.io/rcnode"
// 	tolerationKey         = "recluster.io/node"
// 	nodeSelectorKeyPrefix = "recluster.io/node" // used as <prefix>=<nodeName>
// )

// /* -------------------------------------------------------------------------- */
// /*                              Public entry                                  */
// /* -------------------------------------------------------------------------- */

// // RunStep returns a *deduplicated* list of actions required to converge the
// // cluster state one step closer to the desired policy outcome.

// func RunStep(pods []*corev1.Pod, rcnodes []*reclusterv1.RcNode, policies []*reclusterv1.RcPolicy, now time.Time, scaleCooldown time.Duration) []Action {
// 	// 1. Build quick‑lookup maps for nodes.
// 	nodeByName := make(map[string]*reclusterv1.RcNode)
// 	for _, n := range rcnodes {
// 		nodeByName[n.Name] = n
// 	}

// 	// 2. Identify scheduling‑gated pods that still need placement.
// 	var pending []*corev1.Pod
// 	for _, p := range pods {
// 		if hasGate(p) && p.Annotations[annAssignment] == "" {
// 			pending = append(pending, p)
// 		}
// 	}
// 	klog.Infof("RunStep: total=%d, pending=%d, nodes=%d, policies=%d", len(pods), len(pending), len(rcnodes), len(policies))

// 	// 3. First pass – schedule pods and implicitly mark nodes to power on.
// 	var actions []Action
// 	poweringOn := map[string]struct{}{} // avoid duplicate power‑on per node

// 	for _, pod := range pending {
// 		pol := resolvePolicy(pod, policies)
// 		if pol == nil {
// 			klog.Warningf("no policy for pod %s/%s", pod.Namespace, pod.Name)
// 			continue
// 		}
// 		best := solver.PickBest(rcnodes, pol) // may return sleeping nodes
// 		if best == nil {
// 			klog.Infof("no node fits pod %s/%s under %s", pod.Namespace, pod.Name, pol.Name)
// 			continue
// 		}

// 		// Power‑on action if node is currently Stopped.
// 		if best.Spec.DesiredState == reclusterv1.RcNodeStateStopped {
// 			if _, done := poweringOn[best.Name]; !done {
// 				actions = append(actions, NodePowerAction{
// 					NodeName:     best.Name,
// 					DesiredState: reclusterv1.RcNodeStateRunning,
// 					Reason:       fmt.Sprintf("needed for pod %s/%s", pod.Namespace, pod.Name),
// 				})
// 				poweringOn[best.Name] = struct{}{}
// 			}
// 		}

// 		// Patch pod so scheduler can bind exclusively to best.Name.
// 		actions = append(actions, buildPodScheduleAction(pod, best.Name))
// 	}

// 	// 4. Second pass – power‑off idle nodes honoring boot/cooldown.
// 	idleNodes := findIdleNodes(pods, rcnodes)
// 	for _, n := range idleNodes {
// 		if n.Spec.DesiredState != reclusterv1.RcNodeStateRunning {
// 			continue // already stopping / stopped
// 		}
// 		// do not power‑off if within cooldown since last transition
// 		if n.Status.LastTransition != nil && now.Sub(n.Status.LastTransition.Time) < scaleCooldown {
// 			continue
// 		}
// 		actions = append(actions, NodePowerAction{
// 			NodeName:     n.Name,
// 			DesiredState: reclusterv1.RcNodeStateStopped,
// 			Reason:       "idle node",
// 		})
// 	}

// 	return dedupActions(actions)
// }

// /* -------------------------------------------------------------------------- */
// /*                            helper functions                                */
// /* -------------------------------------------------------------------------- */

// func buildPodScheduleAction(p *corev1.Pod, nodeName string) PodScheduleAction {
// 	nsName := types.NamespacedName{Namespace: p.Namespace, Name: p.Name}
// 	// toleration so pod can land on the tainted node
// 	tol := corev1.Toleration{
// 		Key:      tolerationKey,
// 		Operator: corev1.TolerationOpEqual,
// 		Value:    nodeName,
// 		Effect:   corev1.TaintEffectNoSchedule,
// 	}
// 	// nodeSelector to guarantee exclusivity
// 	selectorKey := fmt.Sprintf("%s", nodeSelectorKeyPrefix)
// 	return PodScheduleAction{
// 		PodRef:       nsName,
// 		TargetNode:   nodeName,
// 		Annotations:  map[string]string{annAssignment: nodeName},
// 		Tolerations:  []corev1.Toleration{tol},
// 		NodeSelector: map[string]string{selectorKey: nodeName},
// 	}
// }

// func hasGate(p *corev1.Pod) bool {
// 	for _, g := range p.Spec.SchedulingGates {
// 		if g.Name == gateKey {
// 			return true
// 		}
// 	}
// 	return false
// }

// func resolvePolicy(pod *corev1.Pod, policies []*reclusterv1.RcPolicy) *reclusterv1.RcPolicy {
// 	for _, pol := range policies {
// 		sel, err := pol.CompiledSelector()
// 		if err != nil {
// 			log.Printf("policy %s bad selector: %v", pol.Name, err)
// 			continue
// 		}
// 		if sel.Matches(labels.Set(pod.Labels)) {
// 			return pol
// 		}
// 	}
// 	return nil
// }

// // findIdleNodes returns nodes with no bound pods (annotation matches).
// func findIdleNodes(pods []*corev1.Pod, nodes []*reclusterv1.RcNode) []*reclusterv1.RcNode {
// 	usage := make(map[string]bool)
// 	for _, p := range pods {
// 		if node := p.Annotations[annAssignment]; node != "" {
// 			usage[node] = true
// 		}
// 	}
// 	var idle []*reclusterv1.RcNode
// 	for _, n := range nodes {
// 		if !usage[n.Name] {
// 			idle = append(idle, n)
// 		}
// 	}
// 	return idle
// }

// // dedupActions removes duplicate NodePowerActions (keep first) while leaving
// // PodScheduleActions untouched and stable‑ordered.
// func dedupActions(in []Action) []Action {
// 	seen := make(map[string]struct{})
// 	var out []Action
// 	for _, a := range in {
// 		if np, ok := a.(NodePowerAction); ok {
// 			key := fmt.Sprintf("%s:%s", np.NodeName, np.DesiredState)
// 			if _, done := seen[key]; done {
// 				continue
// 			}
// 			seen[key] = struct{}{}
// 			out = append(out, a)
// 		} else {
// 			out = append(out, a)
// 		}
// 	}
// 	return out
// }
