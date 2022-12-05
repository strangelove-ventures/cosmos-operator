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

	cosmosv1alpha1 "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/volsnapshot"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ScheduledVolumeSnapshotReconciler reconciles a ScheduledVolumeSnapshot object
type ScheduledVolumeSnapshotReconciler struct {
	client.Client
	recorder record.EventRecorder
	events   chan<- event.GenericEvent
}

func NewScheduledVolumeSnapshotReconciler(
	client client.Client,
	recorder record.EventRecorder,
	events chan<- event.GenericEvent,
) *ScheduledVolumeSnapshotReconciler {
	return &ScheduledVolumeSnapshotReconciler{Client: client, recorder: recorder, events: events}
}

//+kubebuilder:rbac:groups=cosmos.strange.love,resources=scheduledvolumesnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=scheduledvolumesnapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=scheduledvolumesnapshots/finalizers,verbs=update

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

	// TODO: remove me
	logger.V(1).Info("Sending generic event")
	r.events <- event.GenericEvent{Object: crd}

	return finishResult, nil
}

func (r *ScheduledVolumeSnapshotReconciler) updateStatus(ctx context.Context, crd *cosmosv1alpha1.ScheduledVolumeSnapshot) {
	if err := r.Status().Update(ctx, crd); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScheduledVolumeSnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cosmosv1alpha1.ScheduledVolumeSnapshot{}).
		Complete(r)
}
