package webhook

import (
	"context"
	"encoding/json"
	"log"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const gateKey = "recluster-sync/wating-for-recluster-scheduling"

type GateInjector struct {
	decoder admission.Decoder // interface (NOT *admission.Decoder)
}

func (g *GateInjector) Handle(ctx context.Context, req admission.Request) admission.Response {
	log.Printf("GateInjector.Handle: %s %s/%s", req.Operation, req.Name, req.Namespace)
	if req.Operation != admissionv1.Create {
		return admission.Allowed("not a CREATE")
	}

	var pod corev1.Pod
	// v0.20.4: Decode(req, obj) — no context param
	if err := g.decoder.Decode(req, &pod); err != nil {
		return admission.Errored(400, err)
	}

	if skipPod(&pod) {
		return admission.Allowed("skip by rule")
	}

	pod.Spec.SchedulingGates = append(pod.Spec.SchedulingGates,
		corev1.PodSchedulingGate{Name: gateKey})

	marshaled, _ := json.Marshal(&pod)
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
}

// v0.20.4: injector takes the interface
func (g *GateInjector) InjectDecoder(d admission.Decoder) error {
	g.decoder = d
	return nil
}

/* ---- helper: do we skip? --------------------------------------- */
func skipPod(p *corev1.Pod) bool {
	switch p.Namespace {
	case "kube-system", "kube-public", "kube-node-lease":
		return true
	}
	if p.Annotations["recluster.io/policy-skip"] == "true" || p.Spec.HostNetwork {
		return true
	}
	for _, o := range p.OwnerReferences {
		if o.Kind == "DaemonSet" {
			return true
		}
	}
	for _, g := range p.Spec.SchedulingGates {
		if g.Name == gateKey {
			return true
		}
	}
	return false
}
