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
	"net/http"
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/fullnode"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
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

	eventNormal  = "Normal"
	eventWarning = "Warning"
)

// CosmosFullNodeReconciler reconciles a CosmosFullNode object
type CosmosFullNodeReconciler struct {
	client.Client

	configMapControl fullnode.ConfigMapControl
	podControl       fullnode.PodControl
	pvcControl       fullnode.PVCControl
	recorder         record.EventRecorder
	serviceControl   fullnode.ServiceControl
}

// NewFullNode returns a valid CosmosFullNode controller.
func NewFullNode(client client.Client, recorder record.EventRecorder) *CosmosFullNodeReconciler {
	return &CosmosFullNodeReconciler{
		Client:           client,
		configMapControl: fullnode.NewConfigMapControl(client),
		podControl:       fullnode.NewPodControl(client),
		pvcControl:       fullnode.NewPVCControl(client),
		recorder:         recorder,
		serviceControl:   fullnode.NewServiceControl(client),
	}
}

var (
	finishResult  ctrl.Result
	requeueResult = ctrl.Result{RequeueAfter: 3 * time.Second}
)

//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes/finalizers,verbs=update
// Generate RBAC roles to watch and update resources. IMPORTANT!!!! All resource names must be lowercase or cluster role will not work.
//+kubebuilder:rbac:groups="",resources=pods;persistentvolumeclaims;services;configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *CosmosFullNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Entering reconcile loop", "request", req.NamespacedName)

	// Get the CRD
	crd := new(cosmosv1.CosmosFullNode)
	if err := r.Get(ctx, req.NamespacedName, crd); err != nil {
		// Ignore not found errors because can't be fixed by an immediate requeue. We'll have to wait for next notification.
		// Also, will get "not found" error if crd is deleted.
		// No need to explicitly delete resources. Kube GC does so automatically because we set the controller reference
		// for each resource.
		return finishResult, client.IgnoreNotFound(err)
	}

	fullnode.ResetStatus(crd)
	defer r.updateStatus(ctx, crd)

	errs := &kube.ReconcileErrors{}

	// Order of operations is important. E.g. PVCs won't delete unless pods are deleted first.
	// K8S can create pods first even if the PVC isn't ready. Pods won't be in a ready state until PVC is bound.

	// Create or update Services.
	err := r.serviceControl.Reconcile(ctx, logger, crd)
	if err != nil {
		errs.Append(err)
	}

	// Create or update ConfigMaps.
	p2pAddresses, err := fullnode.CollectP2PAddresses(ctx, crd, r)
	if err != nil {
		p2pAddresses = make(fullnode.ExternalAddresses)
		errs.Append(err)
	}
	err = r.configMapControl.Reconcile(ctx, logger, crd, p2pAddresses)
	if err != nil {
		errs.Append(err)
	}

	// Reconcile pods.
	podRequeue, err := r.podControl.Reconcile(ctx, logger, crd)
	if err != nil {
		errs.Append(err)
	}

	// Reconcile pvcs.
	pvcRequeue, err := r.pvcControl.Reconcile(ctx, logger, crd)
	if err != nil {
		errs.Append(err)
	}

	if errs.Any() {
		return r.resultWithErr(crd, errs)
	}

	if podRequeue || pvcRequeue {
		return requeueResult, nil
	}

	// Check final state and requeue if necessary.
	if p2pAddresses.Incomplete() {
		r.recorder.Event(crd, eventNormal, "P2PIncomplete", "Waiting for p2p service IPs or Hostnames to be ready.")
		logger.V(1).Info("Requeueing due to incomplete p2p external addresses")
		// Allow more time to requeue while p2p services create their load balancers.
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	crd.Status.Phase = cosmosv1.FullNodePhaseCompete
	return finishResult, nil
}

func (r *CosmosFullNodeReconciler) resultWithErr(crd *cosmosv1.CosmosFullNode, err kube.ReconcileError) (ctrl.Result, kube.ReconcileError) {
	if err.IsTransient() {
		r.recorder.Event(crd, eventWarning, "ErrorTransient", fmt.Sprintf("%v; retrying.", err))
		crd.Status.StatusMessage = ptr(fmt.Sprintf("Transient error: system is retrying: %v", err))
		return requeueResult, err
	}

	crd.Status.Phase = cosmosv1.FullNodePhaseError
	crd.Status.StatusMessage = ptr(fmt.Sprintf("Unrecoverable error: human intervention required: %v", err))
	r.recorder.Event(crd, eventWarning, "Error", err.Error())
	return finishResult, err
}

func (r *CosmosFullNodeReconciler) updateStatus(ctx context.Context, crd *cosmosv1.CosmosFullNode) {
	if err := r.Status().Update(ctx, crd); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update status")
	}
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

// SetupWithManager sets up the controller with the Manager.
func (r *CosmosFullNodeReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// Index pods.
	err := mgr.GetFieldIndexer().IndexField(
		ctx,
		&corev1.Pod{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.Pod]("CosmosFullNode"),
	)
	if err != nil {
		return fmt.Errorf("pod index field %s: %w", controllerOwnerField, err)
	}

	// Index PVCs.
	err = mgr.GetFieldIndexer().IndexField(
		ctx,
		&corev1.PersistentVolumeClaim{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.PersistentVolumeClaim]("CosmosFullNode"),
	)
	if err != nil {
		return fmt.Errorf("pvc index field %s: %w", controllerOwnerField, err)
	}

	// Index ConfigMaps.
	err = mgr.GetFieldIndexer().IndexField(
		ctx,
		&corev1.ConfigMap{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.ConfigMap]("CosmosFullNode"),
	)
	if err != nil {
		return fmt.Errorf("configmap index field %s: %w", controllerOwnerField, err)
	}

	// Index Services.
	err = mgr.GetFieldIndexer().IndexField(
		ctx,
		&corev1.Service{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.Service]("CosmosFullNode"),
	)
	if err != nil {
		return fmt.Errorf("service index field %s: %w", controllerOwnerField, err)
	}

	cbuilder := ctrl.NewControllerManagedBy(mgr).For(&cosmosv1.CosmosFullNode{})

	// Watch for delete events for certain resources.
	for _, kind := range []*source.Kind{
		{Type: &corev1.Pod{}},
		{Type: &corev1.PersistentVolumeClaim{}},
		{Type: &corev1.ConfigMap{}},
		{Type: &corev1.Service{}},
	} {
		cbuilder.Watches(
			kind,
			&handler.EnqueueRequestForOwner{OwnerType: &cosmosv1.CosmosFullNode{}, IsController: true},
			builder.WithPredicates(&predicate.Funcs{
				DeleteFunc: func(_ event.DeleteEvent) bool { return true },
			}),
		)
	}

	return cbuilder.Complete(r)
}
