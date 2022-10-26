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

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// HostedSnapshotReconciler reconciles a HostedSnapshot object
type HostedSnapshotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cosmos.strange.love,resources=hostedsnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=hostedsnapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cosmos.strange.love,resources=hostedsnapshots/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HostedSnapshot object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *HostedSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("HostedSnapshot")
	logger.V(1).Info("Entering reconcile loop")

	// Get the CRD
	crd := new(cosmosv1.HostedSnapshot)
	if err := r.Get(ctx, req.NamespacedName, crd); err != nil {
		// Ignore not found errors because can't be fixed by an immediate requeue. We'll have to wait for next notification.
		// Also, will get "not found" error if crd is deleted.
		// No need to explicitly delete resources. Kube GC does so automatically because we set the controller reference
		// for each resource.
		return finishResult, client.IgnoreNotFound(err)
	}

	var snapshots snapshotv1.VolumeSnapshotList
	if err := r.List(ctx, &snapshots, client.InNamespace(crd.Namespace)); err != nil {
		// TODO(nix): Report not found in the status object.
		return finishResult, client.IgnoreNotFound(err)
	}

	logger.Info("Found Snapshots", "found", snapshots.Items)

	return finishResult, nil
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
