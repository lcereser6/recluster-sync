package controller

import (
	"context"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	gateKey  = "recluster-sync/waiting-for-node"
	taintKey = "recluster.io/rcnode" // same label & annotation
)

func NewPodReconciler(mgr ctrl.Manager) *PodReconciler {
	return &PodReconciler{Client: mgr.GetClient()}
}

type PodReconciler struct{ client.Client }

func (r *PodReconciler) Reconcile(ctx context.Context,
	req ctrl.Request) (ctrl.Result, error) {

	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 1 — exit early if gate already removed
	if !hasGate(&pod) {
		return ctrl.Result{}, nil
	}

	target := pod.Annotations["recluster.io/rcnode"]
	if target == "" {
		// planner hasn’t annotated yet – requeue soon
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	// 2 — is underlying Node Ready?
	var node corev1.Node
	if err := r.Get(ctx, client.ObjectKey{Name: target}, &node); err != nil {
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}
	if !nodeReady(&node) {
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	// 3 — ensure taint / toleration, then remove gate
	//ensureToleration(&pod, taintKey, target)
	//ensureNodeTaint(&node, taintKey, target)
	if err := r.Patch(ctx, &pod, client.MergeFrom(pod.DeepCopy())); err != nil {
		return ctrl.Result{}, err
	}
	//clearGate(&pod)
	//log that we are patching the pod
	log.Printf("Pod %s/%s is ready for node %s, removing gate", pod.Namespace, pod.Name, target)
	return ctrl.Result{}, r.Patch(ctx, &pod, client.MergeFrom(pod.DeepCopy()))
}

func nodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func hasGate(pod *corev1.Pod) bool {
	for _, g := range pod.Spec.SchedulingGates {
		if g.Name == gateKey {
			return true
		}
	}
	return false
}

func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
