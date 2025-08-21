// internal/state/impl.go
//
// “Live view” of the cluster – pure read-only informers that emit log lines
// whenever something interesting happens.
//
// It implements controller-runtime’s Runnable interface, so main.go can simply
// mgr.Add(stateObj) and forget about it.

package state

import (
	"context"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	// client-go bits
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	// generated clientset for our CRDs
	rcclient "github.com/lcereser6/recluster-sync/apis/client/clientset/versioned"
	rcinformers "github.com/lcereser6/recluster-sync/apis/client/informers/externalversions"
	rclisters "github.com/lcereser6/recluster-sync/apis/client/listers/recluster.com/v1alpha1"
	"github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"
)

// ---------------------------------------------------------------------------
// concrete impl
// ---------------------------------------------------------------------------

type liveState struct {
	factory     informers.SharedInformerFactory   // core/v1
	rcFactory   rcinformers.SharedInformerFactory // our CRDs
	podInf      cache.SharedIndexInformer
	rcnodeInf   cache.SharedIndexInformer
	rcpolicyInf cache.SharedIndexInformer
}

// Pods implements State.
func (s *liveState) Pods() []*v1.Pod {
	pods, err := s.factory.Core().V1().Pods().Lister().List(labels.Everything())
	if err != nil {
		logf.Log.Error(err, "failed to list Pods")
		return nil
	}
	return pods

}

// RcNodes implements State.
func (s *liveState) RcNodes() []*v1alpha1.RcNode {
	nodes, err := s.rcFactory.Recluster().V1alpha1().RcNodes().Lister().List(labels.Everything())
	if err != nil {
		logf.Log.Error(err, "failed to list RcNodes")
		return nil
	}
	return nodes
}

// RcPolicies implements State.
func (s *liveState) RcPolicies() []*v1alpha1.RcPolicy {
	policies, err := s.rcFactory.Recluster().V1alpha1().RcPolicies().Lister().List(labels.Everything())
	if err != nil {
		logf.Log.Error(err, "failed to list RcPolicies")
		return nil
	}
	return policies
}

// New wires informers but does *not* start them (manager will).
func New(restCfg *rest.Config) (State, error) {
	// --- core /v1 client -------------------------------------------------
	cfg, err := rest.InClusterConfig() // or clientcmd.BuildConfigFromFlags
	if err != nil {
		return nil, err
	}
	k8sCS := kubernetes.NewForConfigOrDie(cfg)

	if err != nil {
		return nil, err
	}
	coreFactory := informers.NewSharedInformerFactory(k8sCS, 0)

	// --- CRD client ------------------------------------------------------
	rcCS, err := rcclient.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	rcFactory := rcinformers.NewSharedInformerFactory(rcCS, 0)

	st := &liveState{
		factory:     coreFactory,
		rcFactory:   rcFactory,
		podInf:      coreFactory.Core().V1().Pods().Informer(),
		rcnodeInf:   rcFactory.Recluster().V1alpha1().RcNodes().Informer(),
		rcpolicyInf: rcFactory.Recluster().V1alpha1().RcPolicies().Informer(),
	}

	st.podInf.AddEventHandler(newLoggingHandlers("Pod"))
	st.rcnodeInf.AddEventHandler(newLoggingHandlers("RCNode"))
	st.rcpolicyInf.AddEventHandler(newLoggingHandlers("RCPolicy"))

	return st, nil
}

/* ---------------- controller-runtime Runnable -------------------- */

func (s *liveState) Start(ctx context.Context) error {
	log := logf.FromContext(ctx).WithName("state-runner")

	// start factories -> start ALL informers underneath
	s.factory.Start(ctx.Done())
	s.rcFactory.Start(ctx.Done())

	log.Info("waiting for informer caches to sync …")
	if ok := cache.WaitForCacheSync(
		ctx.Done(),
		s.podInf.HasSynced,
		s.rcnodeInf.HasSynced,
		s.rcpolicyInf.HasSynced,
	); !ok {
		return context.Canceled
	}
	log.Info("caches in-sync – live state ready")

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		log := logf.FromContext(ctx).WithName("state-printer")

		for {
			select {
			case <-ticker.C:

				pods := s.Pods()
				log.Info("Current Pods", "count", len(pods))

				rcNodes := s.RcNodes()
				log.Info("Current RcNodes", "count", len(rcNodes))

				rcPolicies := s.RcPolicies()
				log.Info("Current RcPolicies", "count", len(rcPolicies))

			case <-ctx.Done():
				log.Info("Stopping state printer thread")
				return
			}
		}
	}()

	<-ctx.Done() // block until manager stops
	return nil
}

/* ---------------- public getters ------------------ */

func (s *liveState) RcNodeLister() rclisters.RcNodeLister {
	return s.rcFactory.Recluster().V1alpha1().RcNodes().Lister()
}
func (s *liveState) RcPolicyLister() rclisters.RcPolicyLister {
	return s.rcFactory.Recluster().V1alpha1().RcPolicies().Lister()
}
func (s *liveState) PodInformer() cache.SharedIndexInformer { return s.podInf }

/* ---------------- tiny helpers -------------------- */

func key(obj interface{}) string {
	if k, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
		return k
	}
	return "?"
}

func newLoggingHandlers(resourceName string) cache.ResourceEventHandlerFuncs {
	log := logf.Log.WithName("event-logger")
	addFunc := func(obj interface{}) {
		log.Info("add", "gvk", resourceName, "key", key(obj))
	}
	updateFunc := func(oldObj, newObj interface{}) {
		log.Info("update", "gvk", resourceName, "key", key(newObj))
	}
	deleteFunc := func(obj interface{}) {
		log.Info("delete", "gvk", resourceName, "key", key(obj))
	}

	return cache.ResourceEventHandlerFuncs{
		AddFunc:    addFunc,
		UpdateFunc: updateFunc,
		DeleteFunc: deleteFunc,
	}
}
