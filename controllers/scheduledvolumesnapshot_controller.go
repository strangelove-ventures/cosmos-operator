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
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/volsnapshot"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ScheduledVolumeSnapshotReconciler reconciles a ScheduledVolumeSnapshot object
type ScheduledVolumeSnapshotReconciler struct {
	client.Client
	fullNodeControl    *volsnapshot.FullNodeControl
	recorder           record.EventRecorder
	scheduler          *volsnapshot.Scheduler
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
		fullNodeControl:    volsnapshot.NewFullNodeControl(client.Status()),
		recorder:           recorder,
		scheduler:          volsnapshot.NewScheduler(client),
		volSnapshotControl: volsnapshot.NewVolumeSnapshotControl(client, cosmos.NewSyncedPodFinder(tmClient)),
	}
}

//+kubebuilder:rbac:groups=cosmos.strange.love,resources=scheduledvolumesnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=scheduledvolumesnapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=scheduledvolumesnapshots/finalizers,verbs=update
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=cosmosfullnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;create;delete
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

	retryResult := ctrl.Result{RequeueAfter: 10 * time.Second}

	phase := crd.Status.Phase
	switch phase {
	case cosmosv1alpha1.SnapshotPhaseWaitingForNext:
		logger.Info(string(phase))
		dur, err := r.scheduler.CalcNext(crd)
		if err != nil {
			logger.Error(err, "Failed to find duration until next snapshot")
			r.reportError(crd, "FindNextSnapshotTimeError", err)
			return finishResult, nil // Fatal error. Do not requeue.
		}
		if dur > 0 {
			logger.Info("Requeuing for next snapshot", "duration", dur.String())
			return ctrl.Result{RequeueAfter: dur}, nil
		}
		crd.Status.Phase = cosmosv1alpha1.SnapshotPhaseFindingCandidate

	case cosmosv1alpha1.SnapshotPhaseFindingCandidate:
		logger.Info(string(phase))
		candidate, err := r.volSnapshotControl.FindCandidate(ctx, crd)
		if err != nil {
			logger.Error(err, "Failed to find candidate for volume snapshot")
			r.reportError(crd, "FindCandidateError", err)
			return retryResult, nil
		}
		crd.Status.Phase = cosmosv1alpha1.SnapshotPhaseDeletingPod
		crd.Status.Candidate = &candidate

	case cosmosv1alpha1.SnapshotPhaseDeletingPod:
		logger.Info(string(phase))
		if err := r.fullNodeControl.SignalPodDeletion(ctx, crd); err != nil {
			logger.Error(err, "Failed to patch fullnode status for pod deletion")
			r.reportError(crd, "DeletePodError", err)
			return retryResult, nil
		}
		crd.Status.Phase = cosmosv1alpha1.SnapshotPhaseWaitingForPodDeletion

	case cosmosv1alpha1.SnapshotPhaseWaitingForPodDeletion:
		// TODO: Implement wait logic here.
		logger.Info(string(phase))
		crd.Status.Phase = cosmosv1alpha1.SnapshotPhaseCreating

	case cosmosv1alpha1.SnapshotPhaseCreating:
		candidate := crd.Status.Candidate
		logger.Info(string(phase), "candidatePod", candidate.PodName, "candidatePVC", candidate.PVCName)
		if err := r.volSnapshotControl.CreateSnapshot(ctx, crd, *candidate); err != nil {
			logger.Error(err, "Failed to create volume snapshot")
			r.reportError(crd, "CreateVolumeSnapshotError", err)
			return retryResult, nil
		}
		crd.Status.Phase = cosmosv1alpha1.SnapshotPhaseWaitingForCreation

	case cosmosv1alpha1.SnapshotPhaseWaitingForCreation:
		logger.Info(string(phase))
		ready, err := r.scheduler.IsSnapshotReady(ctx, crd)
		if err != nil {
			logger.Error(err, "Failed to find VolumeSnapshot ready status")
			r.reportError(crd, "VolumeSnapshotReadyError", err)
			return retryResult, nil
		}
		if !ready {
			logger.Info("VolumeSnapshot not ready for use; requeueing")
			return retryResult, nil
		}
		crd.Status.Phase = cosmosv1alpha1.SnapshotPhaseRestorePod

	case cosmosv1alpha1.SnapshotPhaseRestorePod:
		logger.Info(string(phase))
		if err := r.fullNodeControl.SignalPodRestoration(ctx, crd); err != nil {
			logger.Error(err, "Failed to update fullnode status for restoring pod")
			r.reportError(crd, "RestorePodError", err)
			return retryResult, nil
		}
		// Reset to beginning.
		crd.Status.Phase = cosmosv1alpha1.SnapshotPhaseWaitingForNext
	}

	// Updating status in the defer above triggers a new reconcile loop.
	return finishResult, nil
}

func (r *ScheduledVolumeSnapshotReconciler) reportError(crd *cosmosv1alpha1.ScheduledVolumeSnapshot, reason string, err error) {
	r.recorder.Event(crd, eventWarning, reason, err.Error())
	crd.Status.StatusMessage = ptr(fmt.Sprint("Error: ", err))
}

func (r *ScheduledVolumeSnapshotReconciler) updateStatus(ctx context.Context, crd *cosmosv1alpha1.ScheduledVolumeSnapshot) {
	if err := r.Status().Update(ctx, crd); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScheduledVolumeSnapshotReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager) error {
	// We do not have to index Pods by CosmosFullNode because the CosmosFullNodeReconciler already does so.
	// If we repeat it here, the manager returns an error.
	return ctrl.NewControllerManagedBy(mgr).
		For(&cosmosv1alpha1.ScheduledVolumeSnapshot{}).
		Complete(r)
}
