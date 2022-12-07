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

	cosmosv1alpha1 "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/cosmos"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/volsnapshot"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ScheduledVolumeSnapshotReconciler reconciles a ScheduledVolumeSnapshot object
type ScheduledVolumeSnapshotReconciler struct {
	client.Client
	recorder           record.EventRecorder
	volSnapshotControl *volsnapshot.VolumeSnapshotControl
}

var tendermintHTTP = &http.Client{Timeout: 60 * time.Second}

func NewScheduledVolumeSnapshotReconciler(
	client client.Client,
	recorder record.EventRecorder,
) *ScheduledVolumeSnapshotReconciler {
	tmClient := cosmos.NewTendermintClient(tendermintHTTP)
	return &ScheduledVolumeSnapshotReconciler{
		Client:             client,
		recorder:           recorder,
		volSnapshotControl: volsnapshot.NewVolumeSnapshotControl(client, cosmos.NewSyncedPodFinder(tmClient)),
	}
}

//+kubebuilder:rbac:groups=cosmos.strange.love,resources=scheduledvolumesnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=scheduledvolumesnapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=scheduledvolumesnapshots/finalizers,verbs=update
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ScheduledVolumeSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Entering reconcile loop", "request", req.NamespacedName)

	// Get the CRD
	crd := new(cosmosv1alpha1.ScheduledVolumeSnapshot)
	if err := r.Get(ctx, req.NamespacedName, crd); err != nil {
		// Ignore not found errors because can't be fixed by an immediate requeue. We'll have to wait for next notification.
		// Also, will get "not found" error if crd is deleted.
		// No need to explicitly delete resources. Kube GC does so automatically because we set the controller reference
		// for each resource.
		return finishResult, client.IgnoreNotFound(err)
	}

	volsnapshot.ResetStatus(crd)
	defer r.updateStatus(ctx, crd)

	dur, err := volsnapshot.DurationUntilNext(crd, time.Now())
	if err != nil {
		logger.Error(err, "Failed to find duration until next snapshot")
		r.reportError(crd, err)
		return finishResult, nil // Fatal error; do not requeue.
	}

	if dur > 0 {
		logger.V(1).Info("Requeuing for next snapshot", "duration", dur.String())
		return ctrl.Result{RequeueAfter: dur}, nil
	}

	candidate, err := r.volSnapshotControl.FindCandidate(ctx, crd)
	if err != nil {
		logger.Error(err, "Failed to find candidate for volume snapshot")
		r.reportError(crd, err)
		return finishResult, err // Treating as transient so retry with a backoff.
	}

	logger.Info("Found candidate", "candidate", fmt.Sprintf("%+v", candidate))

	return finishResult, nil
}

func (r *ScheduledVolumeSnapshotReconciler) reportError(crd *cosmosv1alpha1.ScheduledVolumeSnapshot, err error) {
	r.recorder.Event(crd, eventWarning, "error", err.Error())
	crd.Status.StatusMessage = ptr(fmt.Sprint("Error:", err))
}

func (r *ScheduledVolumeSnapshotReconciler) updateStatus(ctx context.Context, crd *cosmosv1alpha1.ScheduledVolumeSnapshot) {
	if err := r.Status().Update(ctx, crd); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScheduledVolumeSnapshotReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// Purposefully index pods owned by a CosmosFullNode.
	err := mgr.GetFieldIndexer().IndexField(
		ctx,
		&corev1.Pod{},
		controllerOwnerField,
		kube.IndexOwner[*corev1.Pod]("CosmosFullNode"), // Intentional. This controller does not own any pods.
	)
	if err != nil {
		return fmt.Errorf("pod index field %s: %w", controllerOwnerField, err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&cosmosv1alpha1.ScheduledVolumeSnapshot{}).
		Complete(r)
}
