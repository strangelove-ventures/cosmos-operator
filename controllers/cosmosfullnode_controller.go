/*
Copyright 2024 B-Harvest Corporation.
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

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	"github.com/bharvest-devops/cosmos-operator/internal/cosmos"
	"github.com/bharvest-devops/cosmos-operator/internal/fullnode"
	"github.com/bharvest-devops/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const controllerOwnerField = ".metadata.controller"

// CosmosFullNodeReconciler reconciles a CosmosFullNode object
type CosmosFullNodeReconciler struct {
	client.Client

	cacheController           *cosmos.CacheController
	configMapControl          fullnode.ConfigMapControl
	nodeKeyControl            fullnode.NodeKeyControl
	peerCollector             *fullnode.PeerCollector
	podControl                fullnode.PodControl
	pvcControl                fullnode.PVCControl
	recorder                  record.EventRecorder
	serviceControl            fullnode.ServiceControl
	statusClient              *fullnode.StatusClient
	serviceAccountControl     fullnode.ServiceAccountControl
	clusterRoleControl        fullnode.RoleControl
	clusterRoleBindingControl fullnode.RoleBindingControl
}

// NewFullNode returns a valid CosmosFullNode controller.
func NewFullNode(
	client client.Client,
	recorder record.EventRecorder,
	statusClient *fullnode.StatusClient,
	cacheController *cosmos.CacheController,
) *CosmosFullNodeReconciler {
	return &CosmosFullNodeReconciler{
		Client: client,

		cacheController:           cacheController,
		configMapControl:          fullnode.NewConfigMapControl(client),
		nodeKeyControl:            fullnode.NewNodeKeyControl(client),
		peerCollector:             fullnode.NewPeerCollector(client),
		podControl:                fullnode.NewPodControl(client, cacheController),
		pvcControl:                fullnode.NewPVCControl(client),
		recorder:                  recorder,
		serviceControl:            fullnode.NewServiceControl(client),
		statusClient:              statusClient,
		serviceAccountControl:     fullnode.NewServiceAccountControl(client),
		clusterRoleControl:        fullnode.NewRoleControl(client),
		clusterRoleBindingControl: fullnode.NewRoleBindingControl(client),
	}
}

var (
	stopResult    ctrl.Result
	requeueResult = ctrl.Result{RequeueAfter: 3 * time.Second}
)

//+kubebuilder:rbac:groups=cosmos.bharvest,resources=cosmosfullnodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cosmos.bharvest,resources=cosmosfullnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cosmos.bharvest,resources=cosmosfullnodes/finalizers,verbs=update
// Generate RBAC roles to watch and update resources. IMPORTANT!!!! All resource names must be lowercase or cluster role will not work.
//+kubebuilder:rbac:groups="",resources=pods;persistentvolumeclaims;services;serviceaccounts;configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete;bind;escalate
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch

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
		return stopResult, client.IgnoreNotFound(err)
	}

	reporter := kube.NewEventReporter(logger, r.recorder, crd)

	fullnode.ResetStatus(crd)

	syncInfo := fullnode.SyncInfoStatus(ctx, crd, r.cacheController)

	pvcStatusChanges := fullnode.PVCStatusChanges{}

	defer r.updateStatus(ctx, crd, syncInfo, &pvcStatusChanges)

	errs := &kube.ReconcileErrors{}

	// Order of operations is important. E.g. PVCs won't delete unless pods are deleted first.
	// K8S can create pods first even if the PVC isn't ready. Pods won't be in a ready state until PVC is bound.

	// Create or update Services.
	err := r.serviceControl.Reconcile(ctx, reporter, crd)
	if err != nil {
		errs.Append(err)
	}

	// Reconcile Secrets.
	err = r.nodeKeyControl.Reconcile(ctx, reporter, crd)
	if err != nil {
		errs.Append(err)
	}

	// Find peer information that's used downstream.
	peers, perr := r.peerCollector.Collect(ctx, crd)
	if perr != nil {
		peers = peers.Default()
		errs.Append(perr)
	}

	// Reconcile ConfigMaps.
	configCksums, err := r.configMapControl.Reconcile(ctx, reporter, crd, peers)
	if err != nil {
		errs.Append(err)
	}

	// Reconcile service accounts.
	err = r.serviceAccountControl.Reconcile(ctx, reporter, crd)
	if err != nil {
		errs.Append(err)
	}

	// Reconcile cluster roles.
	err = r.clusterRoleControl.Reconcile(ctx, reporter, crd)
	if err != nil {
		errs.Append(err)
	}

	// Reconcile cluster role bindings.
	err = r.clusterRoleBindingControl.Reconcile(ctx, reporter, crd)
	if err != nil {
		errs.Append(err)
	}

	// Reconcile pods.
	podRequeue, err := r.podControl.Reconcile(ctx, reporter, crd, configCksums, syncInfo)
	if err != nil {
		errs.Append(err)
	}

	// Reconcile pvcs.
	pvcRequeue, err := r.pvcControl.Reconcile(ctx, reporter, crd, &pvcStatusChanges)
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

	for _, i := range peers {
		reporter.Debug(i.ExternalAddress)
	}

	if peers.HasIncompleteExternalAddress() {
		reporter.Info("Requeueing due to incomplete p2p external addresses")
		reporter.RecordInfo("P2PIncomplete", "Waiting for p2p service IPs or Hostnames to be ready.")
		crd.Status.Phase = cosmosv1.FullNodePhaseP2PServices
		// Allow more time to requeue while p2p services create their load balancers.
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	crd.Status.Peers = peers.AllExternal()

	crd.Status.Phase = cosmosv1.FullNodePhaseCompete
	// Requeue to constantly poll consensus state.
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

func (r *CosmosFullNodeReconciler) resultWithErr(crd *cosmosv1.CosmosFullNode, err kube.ReconcileError) (ctrl.Result, kube.ReconcileError) {
	if err.IsTransient() {
		r.recorder.Event(crd, kube.EventWarning, "ErrorTransient", fmt.Sprintf("%v; retrying.", err))
		crd.Status.StatusMessage = ptr(fmt.Sprintf("Transient error: system is retrying: %v", err))
		crd.Status.Phase = cosmosv1.FullNodePhaseTransientError
		return requeueResult, err
	}

	crd.Status.Phase = cosmosv1.FullNodePhaseError
	crd.Status.StatusMessage = ptr(fmt.Sprintf("Unrecoverable error: human intervention required: %v", err))
	r.recorder.Event(crd, kube.EventWarning, "Error", err.Error())
	return stopResult, err
}

func (r *CosmosFullNodeReconciler) updateStatus(
	ctx context.Context,
	crd *cosmosv1.CosmosFullNode,
	syncInfo map[string]*cosmosv1.SyncInfoPodStatus,
	pvcStatusChanges *fullnode.PVCStatusChanges,
) {
	if err := r.statusClient.SyncUpdate(ctx, client.ObjectKeyFromObject(crd), func(status *cosmosv1.FullNodeStatus) {
		status.ObservedGeneration = crd.Status.ObservedGeneration
		status.Phase = crd.Status.Phase
		status.StatusMessage = crd.Status.StatusMessage
		status.Peers = crd.Status.Peers
		status.SyncInfo = syncInfo
		for k, v := range syncInfo {
			if v.Height != nil && *v.Height > 0 {
				if status.Height == nil {
					status.Height = make(map[string]uint64)
				}
				status.Height[k] = *v.Height + 1 // we want the block that is going through consensus, not the committed one.
			}
		}
		if status.SelfHealing.PVCAutoScale != nil {
			for _, k := range pvcStatusChanges.Deleted {
				delete(status.SelfHealing.PVCAutoScale, k)
			}
		}
	}); err != nil {
		log.FromContext(ctx).Error(err, "Failed to patch status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *CosmosFullNodeReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// Index pods.
	err := mgr.GetFieldIndexer().IndexField(
		ctx,
		&corev1.Pod{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.Pod](cosmosv1.CosmosFullNodeController),
	)
	if err != nil {
		return fmt.Errorf("pod index field %s: %w", controllerOwnerField, err)
	}

	// Index PVCs.
	err = mgr.GetFieldIndexer().IndexField(
		ctx,
		&corev1.PersistentVolumeClaim{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.PersistentVolumeClaim](cosmosv1.CosmosFullNodeController),
	)
	if err != nil {
		return fmt.Errorf("pvc index field %s: %w", controllerOwnerField, err)
	}

	// Index ConfigMaps.
	err = mgr.GetFieldIndexer().IndexField(
		ctx,
		&corev1.ConfigMap{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.ConfigMap](cosmosv1.CosmosFullNodeController),
	)
	if err != nil {
		return fmt.Errorf("configmap index field %s: %w", controllerOwnerField, err)
	}

	// Index Secrets.
	err = mgr.GetFieldIndexer().IndexField(
		ctx,
		&corev1.Secret{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.Secret](cosmosv1.CosmosFullNodeController),
	)
	if err != nil {
		return fmt.Errorf("secret index field %s: %w", controllerOwnerField, err)
	}

	// Index Services.
	err = mgr.GetFieldIndexer().IndexField(
		ctx,
		&corev1.Service{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.Service](cosmosv1.CosmosFullNodeController),
	)
	if err != nil {
		return fmt.Errorf("service index field %s: %w", controllerOwnerField, err)
	}

	cbuilder := ctrl.NewControllerManagedBy(mgr).For(&cosmosv1.CosmosFullNode{})

	// Watch for delete events for certain resources.
	for _, object := range []client.Object{
		&corev1.Pod{},
		&corev1.PersistentVolumeClaim{},
		&corev1.ConfigMap{},
		&corev1.Service{},
		&corev1.Secret{},
	} {
		cbuilder.Watches(
			object,
			handler.EnqueueRequestForOwner(r.Scheme(), r.Client.RESTMapper(), &cosmosv1.CosmosFullNode{}),
			builder.WithPredicates(&predicate.Funcs{
				DeleteFunc: func(_ event.DeleteEvent) bool { return true },
			}),
		)
	}

	return cbuilder.Complete(r)
}
