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

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/snapshot"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// HostedSnapshotReconciler reconciles a HostedSnapshot object.
type HostedSnapshotReconciler struct {
	client.Client
	recorder record.EventRecorder
}

// NewHostedSnapshot returns a valid controller.
func NewHostedSnapshot(client client.Client, recorder record.EventRecorder) *HostedSnapshotReconciler {
	return &HostedSnapshotReconciler{
		Client:   client,
		recorder: recorder,
	}
}

// Requeue on a period interval to detect when it's time to run a snapshot job.
var requeueSnapshot = ctrl.Result{RequeueAfter: 60 * time.Second}

//+kubebuilder:rbac:groups=cosmos.strange.love,resources=hostedsnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=hostedsnapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=hostedsnapshots/finalizers,verbs=update
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups="batch",resources=jobs,verbs=get;list;watch;create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *HostedSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Entering reconcile loop")

	crd := new(cosmosv1.HostedSnapshot)
	if err := r.Get(ctx, req.NamespacedName, crd); err != nil {
		// Ignore not found errors because can't be fixed by an immediate requeue. We'll have to wait for next notification.
		// Also, will get "not found" error if crd is deleted.
		// No need to explicitly delete resources. Kube GC does so automatically because we set the controller reference
		// for each resource.
		return requeueSnapshot, client.IgnoreNotFound(err)
	}

	crd.Status.ObservedGeneration = crd.Generation
	crd.Status.StatusMessage = nil
	defer r.updateStatus(ctx, crd)

	// Find active job, if any.
	found, active, err := snapshot.FindActiveJob(ctx, r, crd)
	if err != nil {
		r.reportErr(logger, crd, err)
		return requeueSnapshot, nil
	}

	// Update status if job still active and requeue.
	if found {
		crd.Status.JobHistory = snapshot.UpdateJobStatus(crd.Status.JobHistory, active.Status)
		return requeueSnapshot, nil
	}

	err = r.createResources(ctx, crd)
	if err != nil {
		r.reportErr(logger, crd, err)
		return requeueSnapshot, nil
	}

	crd.Status.JobHistory = snapshot.AddJobStatus(crd.Status.JobHistory, batchv1.JobStatus{})
	// Requeue quickly so we get updated job status on the next reconcile.
	return ctrl.Result{RequeueAfter: time.Second}, nil
}

func (r *HostedSnapshotReconciler) createResources(ctx context.Context, crd *cosmosv1.HostedSnapshot) error {
	logger := log.FromContext(ctx)

	// Find most recent VolumeSnapshot.
	recent, err := snapshot.RecentVolumeSnapshot(ctx, r, crd)
	if err != nil {
		return err
	}
	logger.V(1).Info("Found VolumeSnapshot", "name", recent.Name)

	// Create PVCs.
	if err = snapshot.NewCreator(r, func() ([]*corev1.PersistentVolumeClaim, error) {
		return snapshot.BuildPVCs(crd, recent)
	}).Create(ctx, crd); err != nil {
		return err
	}

	// Create jobs.
	return snapshot.NewCreator(r, func() ([]*batchv1.Job, error) {
		return snapshot.BuildJobs(crd), nil
	}).Create(ctx, crd)
}

func (r *HostedSnapshotReconciler) reportErr(logger logr.Logger, crd *cosmosv1.HostedSnapshot, err error) {
	logger.Error(err, "An error occurred")
	msg := err.Error()
	r.recorder.Event(crd, eventWarning, "error", msg)
	crd.Status.StatusMessage = &msg
}

func (r *HostedSnapshotReconciler) updateStatus(ctx context.Context, crd *cosmosv1.HostedSnapshot) {
	if err := r.Status().Update(ctx, crd); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *HostedSnapshotReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// Index all VolumeSnapshots. Controller does not own any because it does not create them.
	if err := mgr.GetFieldIndexer().IndexField(
		ctx,
		&snapshotv1.VolumeSnapshot{},
		".metadata.name",
		func(object client.Object) []string {
			return []string{object.GetName()}
		},
	); err != nil {
		return fmt.Errorf("VolumeSnapshot index: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&cosmosv1.HostedSnapshot{}).
		Complete(r)
}
