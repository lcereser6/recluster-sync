package solver

import (
	"testing"

	rcv1 "github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPickBest(t *testing.T) {
	nodes := []rcv1.Rcnode{
		{ObjectMeta: metav1.ObjectMeta{Name: "tiny"},
			Spec: rcv1.RcnodeSpec{CPUCores: 2, MemoryGiB: 4, BootSeconds: 30}},
		{ObjectMeta: metav1.ObjectMeta{Name: "fat"},
			Spec: rcv1.RcnodeSpec{CPUCores: 8, MemoryGiB: 32, BootSeconds: 90}},
	}

	// minimise boot-time, mildly care about power (proxy by cpu)
	pol := &rcv1.RcPolicy{Spec: rcv1.RcPolicySpec{
		Metrics: []rcv1.PolicyMetric{
			{Key: "boot", Weight: +1},
			{Key: "cpu", Weight: +0.2},
		},
		HardConstraints: []rcv1.PolicyConstraint{
			{Expression: "ram >= 4"},
		},
	}}

	got, err := PickBest(pol, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Name != "tiny" {
		t.Fatalf("expected tiny, got %v", got)
	}
}
