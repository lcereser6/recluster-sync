package controller

import (
	"context"

	reclusterv1 "github.com/lcereser6/recluster-sync/apis/recluster.com/v1alpha1"
	"github.com/lcereser6/recluster-sync/internal/backend"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RcNodeReconciler struct {
	client.Client
	be backend.Backend
}

func NewRcNodeReconciler(mgr ctrl.Manager, be backend.Backend) *RcNodeReconciler {
	return &RcNodeReconciler{Client: mgr.GetClient(), be: be}
}

func (r *RcNodeReconciler) Reconcile(ctx context.Context,
	req ctrl.Request) (ctrl.Result, error) {

	var rc reclusterv1.RcNode
	if err := r.Get(ctx, req.NamespacedName, &rc); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if err := r.be.Reconcile(ctx, &rc); err != nil {
		return ctrl.Result{}, err // retry on backend error
	}
	// backend did its job → update observed status if you like
	return ctrl.Result{}, nil
}

func (r *RcNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&reclusterv1.RcNode{}).
		Complete(r)
}
