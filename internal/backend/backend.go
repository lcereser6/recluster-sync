package backend

import (
	"context"
	"fmt"

	rcv1 "github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// Backend reconciles ONE Rcnode.
type Backend interface {
	// Reconcile must make the real world match rc.Spec.DesiredState
	// – create / patch / delete resources as needed.
	Reconcile(ctx context.Context, rc *rcv1.Rcnode) error
}

// -----------------------------------------------------------------------------
// Factory helper – returns the concrete impl selected by MODE env var
// -----------------------------------------------------------------------------
func New(mode string, k8s kubernetes.Interface) (Backend, error) {
	//loga
	klog.Infof("Backend used: %q", mode)
	switch mode {
	case "kwok":
		return NewKwokBackend(k8s), nil
	case "test":
		return nil, nil
	case "prod":
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown MODE=%q", mode)
	}
}
