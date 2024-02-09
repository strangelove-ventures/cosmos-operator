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
	"errors"
	"fmt"
	"time"

	cosmosalpha "github.com/bharvest-devops/cosmos-operator/api/v1alpha1"
	"github.com/bharvest-devops/cosmos-operator/internal/kube"
	"github.com/bharvest-devops/cosmos-operator/internal/statefuljob"
	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var errMissingVolSnapCRD = errors.New("cluster does not have VolumeSnapshot CRDs installed")

// IndexVolumeSnapshots indexes all VolumeSnapshots by name. Exposed as a separate method so caller can
// test for presence of VolumeSnapshot CRDs in the cluster.
func IndexVolumeSnapshots(ctx context.Context, mgr ctrl.Manager) error {
	// Index all VolumeSnapshots. Controller does not own any because it does not create them.
	if err := mgr.GetFieldIndexer().IndexField(
		ctx,
		&snapshotv1.VolumeSnapshot{},
		".metadata.name",
		func(object client.Object) []string {
			return []string{object.GetName()}
		},
	); err != nil {
		return fmt.Errorf("volume snapshot index: %w", err)
	}
	return nil
}

// StatefulJobReconciler reconciles a StatefulJob object.
type StatefulJobReconciler struct {
	client.Client
	recorder              record.EventRecorder
	missingVolSnapshotCRD bool
}

// NewStatefulJob returns a valid controller. If missingVolSnapCRD is true, the controller errors on every reconcile loop
// and will not function.
func NewStatefulJob(client client.Client, recorder record.EventRecorder, missingVolSnapCRD bool) *StatefulJobReconciler {
	return &StatefulJobReconciler{
		Client:                client,
		recorder:              recorder,
		missingVolSnapshotCRD: missingVolSnapCRD,
	}
}

var requeueStatefulJob = ctrl.Result{RequeueAfter: 60 * time.Second}

//+kubebuilder:rbac:groups=cosmos.bharvest,resources=statefuljobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cosmos.bharvest,resources=statefuljobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cosmos.bharvest,resources=statefuljobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups="batch",resources=jobs,verbs=get;list;watch;create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *StatefulJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Entering reconcile loop")

	crd := new(cosmosalpha.StatefulJob)
	if err := r.Get(ctx, req.NamespacedName, crd); err != nil {
		// Ignore not found errors because can't be fixed by an immediate requeue. We'll have to wait for next notification.
		// Also, will get "not found" error if crd is deleted.
		// No need to explicitly delete resources. Kube GC does so automatically because we set the controller reference
		// for each resource.
		return requeueStatefulJob, kube.IgnoreNotFound(err)
	}

	if r.missingVolSnapshotCRD {
		r.reportErr(logger, crd, errMissingVolSnapCRD)
		return ctrl.Result{}, nil
	}

	crd.Status.ObservedGeneration = crd.Generation
	crd.Status.StatusMessage = nil
	defer r.updateStatus(ctx, crd)

	// Find active job, if any.
	found, active, err := statefuljob.FindActiveJob(ctx, r, crd)
	if err != nil {
		r.reportErr(logger, crd, err)
		return requeueStatefulJob, nil
	}

	// Update status if job still present and requeue.
	if found {
		crd.Status.JobHistory = statefuljob.UpdateJobStatus(crd.Status.JobHistory, active.Status)
		// Requeue quickly to minimize races where job is deleted before we can grab final status.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Delete any existing PVCs so we can create new ones.
	r.deletePVCs(ctx, crd)

	// Check if we need to fire new job/pvc combos.
	if !statefuljob.ReadyForSnapshot(crd, time.Now()) {
		return requeueResult, nil
	}

	// Create new jobs and pvcs.
	err = r.createResources(ctx, crd)
	if err != nil {
		r.reportErr(logger, crd, err)
		return requeueStatefulJob, nil
	}

	crd.Status.JobHistory = statefuljob.AddJobStatus(crd.Status.JobHistory, batchv1.JobStatus{})
	// Requeue quickly so we get updated job status on the next reconcile.
	return ctrl.Result{RequeueAfter: time.Second}, nil
}

func (r *StatefulJobReconciler) createResources(ctx context.Context, crd *cosmosalpha.StatefulJob) error {
	logger := log.FromContext(ctx)

	// Find most recent VolumeSnapshot.
	recent, err := kube.RecentVolumeSnapshot(ctx, r, crd.Namespace, crd.Spec.Selector)
	if err != nil {
		return err
	}
	logger.V(1).Info("Found VolumeSnapshot", "name", recent.Name)

	// Create PVCs.
	if err = statefuljob.NewCreator(r, func() ([]*corev1.PersistentVolumeClaim, error) {
		return statefuljob.BuildPVCs(crd, recent)
	}).Create(ctx, crd); err != nil {
		return err
	}

	// Create jobs.
	return statefuljob.NewCreator(r, func() ([]*batchv1.Job, error) {
		return statefuljob.BuildJobs(crd), nil
	}).Create(ctx, crd)
}

func (r *StatefulJobReconciler) deletePVCs(ctx context.Context, crd *cosmosalpha.StatefulJob) {
	logger := log.FromContext(ctx)

	var pvc corev1.PersistentVolumeClaim
	pvc.Namespace = crd.Namespace
	pvc.Name = statefuljob.ResourceName(crd)

	err := r.Delete(ctx, &pvc)
	switch {
	case kube.IsNotFound(err):
		return
	case err != nil:
		r.reportErr(logger, crd, err)
	default:
		logger.Info("Deleted PVC", "resource", pvc.Name)
	}
}

func (r *StatefulJobReconciler) reportErr(logger logr.Logger, crd *cosmosalpha.StatefulJob, err error) {
	logger.Error(err, "An error occurred")
	msg := err.Error()
	r.recorder.Event(crd, kube.EventWarning, "Error", msg)
	crd.Status.StatusMessage = &msg
}

func (r *StatefulJobReconciler) updateStatus(ctx context.Context, crd *cosmosalpha.StatefulJob) {
	if err := r.Status().Update(ctx, crd); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update status")
	}
}

// SetupWithManager sets up the controller with the Manager. IndexVolumeSnapshots should be called first.
func (r *StatefulJobReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	cbuilder := ctrl.NewControllerManagedBy(mgr).For(&cosmosalpha.StatefulJob{})

	// Watch for delete events for jobs.
	cbuilder.Watches(
		&batchv1.Job{},
		&handler.EnqueueRequestForObject{},
		builder.WithPredicates(
			statefuljob.LabelSelectorPredicate(),
			statefuljob.DeletePredicate(),
		),
	)

	return cbuilder.Complete(r)
}
