package fullnode

import (
	"context"
	"fmt"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PVCControl reconciles volumes for a CosmosFullNode.
// Unlike StatefulSet, PVCControl will update volumes by deleting and recreating volumes.
type PVCControl struct {
	client               Client
	recentVolumeSnapshot func(ctx context.Context, lister kube.Lister, namespace string, selector map[string]string) (*snapshotv1.VolumeSnapshot, error)
}

// NewPVCControl returns a valid PVCControl
func NewPVCControl(client Client) PVCControl {
	return PVCControl{
		client:               client,
		recentVolumeSnapshot: kube.RecentVolumeSnapshot,
	}
}

// Reconcile is the control loop for PVCs. The bool return value, if true, indicates the controller should requeue
// the request.
func (control PVCControl) Reconcile(ctx context.Context, reporter kube.Reporter, crd *cosmosv1.CosmosFullNode) (bool, kube.ReconcileError) {
	// Find any existing pvcs for this CRD.
	var vols corev1.PersistentVolumeClaimList
	if err := control.client.List(ctx, &vols,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
		SelectorLabels(crd),
	); err != nil {
		return false, kube.TransientError(fmt.Errorf("list existing pvcs: %w", err))
	}

	var (
		currentPVCs = ptrSlice(vols.Items)
		wantPVCs    = BuildPVCs(crd)
		diffed      = diff.New(currentPVCs, wantPVCs)
	)

	var dataSource *corev1.TypedLocalObjectReference
	if len(diffed.Creates()) > 0 {
		dataSource = control.findDataSource(ctx, reporter, crd)
	}

	for _, pvc := range diffed.Creates() {
		pvc.Spec.DataSource = dataSource
		reporter.Info("Creating pvc", "pvc", pvc.Name)
		if err := ctrl.SetControllerReference(crd, pvc, control.client.Scheme()); err != nil {
			return true, kube.TransientError(fmt.Errorf("set controller reference on pvc %q: %w", pvc.Name, err))
		}
		if err := control.client.Create(ctx, pvc); kube.IgnoreAlreadyExists(err) != nil {
			return true, kube.TransientError(fmt.Errorf("create pvc %q: %w", pvc.Name, err))
		}
	}

	var deletes int
	if !control.shouldRetain(crd) {
		for _, pvc := range diffed.Deletes() {
			reporter.Info("Deleting pvc", "pvc", pvc.Name)
			if err := control.client.Delete(ctx, pvc, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
				return true, kube.TransientError(fmt.Errorf("delete pvc %q: %w", pvc.Name, err))
			}
		}
		deletes = len(diffed.Deletes())
	}

	if deletes+len(diffed.Creates()) > 0 {
		// Scaling happens first; then updates. So requeue to handle updates after scaling finished.
		return true, nil
	}

	if len(diffed.Updates()) == 0 {
		return false, nil
	}

	if _, unbound := lo.Find(currentPVCs, func(pvc *corev1.PersistentVolumeClaim) bool {
		return pvc.Status.Phase != corev1.ClaimBound
	}); unbound {
		return true, nil
	}

	// PVCs have many immutable fields, so only update the storage size.
	for _, pvc := range diffed.Updates() {
		reporter.Info("Patching pvc", "pvc", pvc.Name)
		patch := corev1.PersistentVolumeClaim{
			ObjectMeta: pvc.ObjectMeta,
			TypeMeta:   pvc.TypeMeta,
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: pvc.Spec.Resources,
			},
		}
		if err := control.client.Patch(ctx, &patch, client.Merge); err != nil {
			reporter.Error(err, "PVC patch failed", "pvc", pvc.Name)
			reporter.RecordError("PVCPatchFailed", err)
			continue
		}
	}

	return false, nil
}

func (control PVCControl) shouldRetain(crd *cosmosv1.CosmosFullNode) bool {
	if policy := crd.Spec.RetentionPolicy; policy != nil {
		return *policy == cosmosv1.RetentionPolicyRetain
	}
	return false
}

func (control PVCControl) findDataSource(ctx context.Context, reporter kube.Reporter, crd *cosmosv1.CosmosFullNode) *corev1.TypedLocalObjectReference {
	if ds := crd.Spec.VolumeClaimTemplate.DataSource; ds != nil {
		return ds
	}
	spec := crd.Spec.VolumeClaimTemplate.AutoDataSource
	if spec == nil {
		return nil
	}
	selector := spec.VolumeSnapshotSelector
	if len(selector) == 0 {
		return nil
	}
	found, err := control.recentVolumeSnapshot(ctx, control.client, crd.Namespace, selector)
	if err != nil {
		reporter.Error(err, "Failed to find VolumeSnapshot for AutoDataSource")
		reporter.RecordError("AutoDataSourceFindSnapshot", err)
		return nil
	}

	reporter.RecordInfo("AutoDataSource", "Using recent VolumeSnapshot for PVC data source")
	return &corev1.TypedLocalObjectReference{
		APIGroup: ptr("snapshot.storage.k8s.io"),
		Kind:     "VolumeSnapshot",
		Name:     found.Name,
	}
}
