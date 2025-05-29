package state

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	rcv1 "github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type impl struct {
	/* immutable */
	k8s    kubernetes.Interface
	dyn    dynamic.Interface
	scheme *runtime.Scheme

	/* live caches (guarded by mu) */
	mu      sync.RWMutex
	rcNodes map[string]*rcv1.Rcnode
	pods    map[types.NamespacedName]*corev1.Pod

	/* informers */
	rcInf  cache.SharedIndexInformer
	podInf cache.SharedIndexInformer
}

func (s *impl) Start(ctx context.Context) error {
	s.rcNodes = map[string]*rcv1.Rcnode{}
	s.pods = map[types.NamespacedName]*corev1.Pod{}

	f := informers.NewSharedInformerFactory(s.k8s, resyncPeriod)

	// --- Rcnode CRD informer (dynamic) -----------------------------------
	gvr := rcv1.GroupVersion.WithResource("rcnodes")
	s.rcInf = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options meta.ListOptions) (runtime.Object, error) {
				return s.dyn.Resource(gvr).List(ctx, options)
			},
			WatchFunc: func(options meta.ListOptions) (watch.Interface, error) {
				return s.dyn.Resource(gvr).Watch(ctx, options)
			},
		},
		&rcv1.Rcnode{}, 0, cache.Indexers{},
	)
	s.rcInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    s.onRcnodeAdd,
		UpdateFunc: func(_, newObj interface{}) { s.onRcnodeAdd(newObj) },
		DeleteFunc: s.onRcnodeDel,
	})

	// --- Pods (all namespaces) -------------------------------------------
	s.podInf = f.Core().V1().Pods().Informer()
	s.podInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    s.onPodAdd,
		UpdateFunc: func(_, newObj interface{}) { s.onPodAdd(newObj) },
		DeleteFunc: s.onPodDel,
	})

	go s.rcInf.Run(ctx.Done())
	go f.Start(ctx.Done())

	// block until both have synced
	if !cache.WaitForCacheSync(ctx.Done(), s.rcInf.HasSynced, s.podInf.HasSynced) {
		return fmt.Errorf("state: informer sync failed")
	}
	klog.Info("state: informer caches in-sync")
	return nil
}

/* ---------------------------- callbacks ---------------------------------- */

func (c *impl) onRcnodeAdd(obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("rcnode add: not Unstructured, got %T", obj)
		return
	}
	var rc rcv1.Rcnode
	if err := runtime.DefaultUnstructuredConverter.
		FromUnstructured(u.Object, &rc); err != nil {
		klog.ErrorS(err, "cannot convert Rcnode")
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rcNodes[rc.Name] = &rc
}
func (s *impl) onRcnodeDel(obj interface{}) {
	rc := obj.(*rcv1.Rcnode)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rcNodes, rc.Name)
}

func (s *impl) onPodAdd(obj interface{}) {
	p := obj.(*corev1.Pod)
	key := types.NamespacedName{Namespace: p.Namespace, Name: p.Name}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pods[key] = p.DeepCopy()
}
func (s *impl) onPodDel(obj interface{}) {
	p := obj.(*corev1.Pod)
	key := types.NamespacedName{Namespace: p.Namespace, Name: p.Name}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pods, key)
}

/* -------------------------- public snapshots ----------------------------- */

func (s *impl) RcNodes() []rcv1.Rcnode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]rcv1.Rcnode, 0, len(s.rcNodes))
	for _, rc := range s.rcNodes {
		out = append(out, *rc.DeepCopy())
	}
	return out
}

func (s *impl) UnschedulablePods() []corev1.Pod {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []corev1.Pod{}
	for _, p := range s.pods {
		if isUnschedulable(p) {
			out = append(out, *p.DeepCopy())
		}
	}
	return out
}

func (s *impl) PodByKey(k types.NamespacedName) *corev1.Pod {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.pods[k]; ok {
		return p.DeepCopy()
	}
	return nil
}

/* ------------------------- mutation helpers ------------------------------ */

func (s *impl) UpdatePodAnnotations(p *corev1.Pod, add map[string]string) error {
	after := p.DeepCopy()
	if after.Annotations == nil {
		after.Annotations = map[string]string{}
	}
	for k, v := range add {
		after.Annotations[k] = v
	}
	beforeJSON, _ := json.Marshal(p)
	afterJSON, _ := json.Marshal(after)

	patch, _ := strategicpatch.CreateTwoWayMergePatch(beforeJSON, afterJSON, corev1.Pod{})
	_, err := s.k8s.CoreV1().Pods(p.Namespace).Patch(context.TODO(), p.Name,
		types.StrategicMergePatchType, patch, meta.PatchOptions{})
	return err
}

/* ------------------------------ helpers ---------------------------------- */

func isUnschedulable(p *corev1.Pod) bool {
	if p.Spec.NodeName != "" {
		return false // already scheduled
	}
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodScheduled &&
			c.Status == corev1.ConditionFalse &&
			c.Reason == corev1.PodReasonUnschedulable {
			return true
		}
	}
	return false
}
