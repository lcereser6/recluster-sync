package state

import (
	"context"
	"time"

	rcv1 "github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const resyncPeriod = 5 * time.Minute

// New returns a ready-to-start State. Use the same *rest.Config the
// controller already has.
func New(cfg *rest.Config) (State, error) {
	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &impl{
		k8s:    k8sClient,
		dyn:    dyn,
		scheme: runtime.NewScheme(), // not strictly needed yet
	}, nil
}

/* -------------------------- interface definition ------------------------- */

type State interface {
	Start(ctx context.Context) error

	RcNodes() []rcv1.Rcnode
	UnschedulablePods() []corev1.Pod
	PodByKey(key types.NamespacedName) *corev1.Pod

	UpdatePodAnnotations(pod *corev1.Pod, add map[string]string) error
}
