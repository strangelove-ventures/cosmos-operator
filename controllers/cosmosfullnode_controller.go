/*
Copyright 2022 Strangelove Ventures LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/fullnode"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerOwnerField = ".metadata.controller"
)

// CosmosFullNodeReconciler reconciles a CosmosFullNode object
type CosmosFullNodeReconciler struct {
	client.Client

	configMapControl fullnode.ConfigMapControl
	podControl       fullnode.PodControl
	pvcControl       fullnode.PVCControl
}

// NewFullNode returns a valid CosmosFullNode controller.
func NewFullNode(client client.Client) *CosmosFullNodeReconciler {
	return &CosmosFullNodeReconciler{
		Client:           client,
		configMapControl: fullnode.NewConfigMapControl(client),
		podControl:       fullnode.NewPodControl(client),
		pvcControl:       fullnode.NewPVCControl(client),
	}
}

var (
	finishResult  ctrl.Result
	requeueResult = ctrl.Result{RequeueAfter: 3 * time.Second}
)

//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes/finalizers,verbs=update
// Generate RBAC roles to watch and update Pods
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;watch;list;delete
//+kubebuilder:rbac:groups="",resources=pvcs,verbs=get;watch;list;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *CosmosFullNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the CRD
	var crd cosmosv1.CosmosFullNode
	if err := r.Get(ctx, req.NamespacedName, &crd); err != nil {
		// Ignore not found errors because can't be fixed by an immediate requeue. We'll have to wait for next notification.
		// Also, will get "not found" error if crd is deleted.
		// No need to explicitly delete resources. Kube GC does so automatically because we set the controller reference
		// for each resource.
		return finishResult, client.IgnoreNotFound(err)
	}

	// Order of operations is important here for deletion. PVCs won't delete unless pods are deleted first.
	// K8S can create pods first even if the PVC isn't ready. Pods won't be in a ready state until PVC is bound.

	// Create or update ConfigMap.
	requeue, err := r.configMapControl.Reconcile(ctx, logger, &crd)
	if err != nil {
		return r.resultWithErr(err)
	}
	if requeue {
		return requeueResult, nil
	}

	// Reconcile pods.
	requeue, err = r.podControl.Reconcile(ctx, logger, &crd)
	if err != nil {
		return r.resultWithErr(err)
	}
	if requeue {
		return requeueResult, nil
	}

	// Reconcile pvcs.
	requeue, err = r.pvcControl.Reconcile(ctx, logger, &crd)
	if err != nil {
		return r.resultWithErr(err)
	}
	if requeue {
		return requeueResult, nil
	}

	return finishResult, nil
}

func (r *CosmosFullNodeReconciler) resultWithErr(err kube.ReconcileError) (ctrl.Result, kube.ReconcileError) {
	if err.IsTransient() {
		return requeueResult, err
	}
	return finishResult, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *CosmosFullNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Index pods.
	err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.Pod{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.Pod]("CosmosFullNode"),
	)
	if err != nil {
		return fmt.Errorf("pod index field %s: %w", controllerOwnerField, err)
	}

	// Index PVCs.
	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.PersistentVolumeClaim{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.PersistentVolumeClaim]("CosmosFullNode"),
	)
	if err != nil {
		return fmt.Errorf("pvc index field %s: %w", controllerOwnerField, err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&cosmosv1.CosmosFullNode{}).
		// Watch some pod events to queue requests for pods owned by CosmosFullNode controller.
		Watches(
			&source.Kind{Type: &corev1.Pod{}},
			&handler.EnqueueRequestForOwner{OwnerType: &cosmosv1.CosmosFullNode{}, IsController: true},
			builder.WithPredicates(&predicate.Funcs{
				DeleteFunc: func(_ event.DeleteEvent) bool { return true },
			},
			),
		).
		// Watch some pvc events to queue requests for pvcs owned by CosmosFullNode controller.
		Watches(
			&source.Kind{Type: &corev1.PersistentVolumeClaim{}},
			&handler.EnqueueRequestForOwner{OwnerType: &cosmosv1.CosmosFullNode{}, IsController: true},
			builder.WithPredicates(&predicate.Funcs{
				DeleteFunc: func(_ event.DeleteEvent) bool { return true },
			},
			),
		).
		Complete(r)
}
