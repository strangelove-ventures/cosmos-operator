package fullnode

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type pvcDiffer interface {
	Creates() []*corev1.PersistentVolumeClaim
	Updates() []*corev1.PersistentVolumeClaim
	Deletes() []*corev1.PersistentVolumeClaim
}

// PVCControl reconciles volumes for a CosmosFullNode.
// Unlike StatefulSet, PVCControl will update volumes by deleting and recreating volumes.
type PVCControl struct {
	client               Client
	diffFactory          func(ordinalAnnotationKey, revisionLabelKey string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer
	recentVolumeSnapshot func(ctx context.Context, lister kube.Lister, namespace string, selector map[string]string) (*snapshotv1.VolumeSnapshot, error)
}

// NewPVCControl returns a valid PVCControl
func NewPVCControl(client Client) PVCControl {
	return PVCControl{
		client:               client,
		recentVolumeSnapshot: kube.RecentVolumeSnapshot,
		diffFactory: func(ordinalAnnotationKey, revisionLabelKey string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer {
			return kube.NewOrdinalDiff(ordinalAnnotationKey, revisionLabelKey, current, want)
		},
	}
}

// Reconcile is the control loop for PVCs. The bool return value, if true, indicates the controller should requeue
// the request.
func (control PVCControl) Reconcile(ctx context.Context, log logr.Logger, crd *cosmosv1.CosmosFullNode) (bool, kube.ReconcileError) {
	// TODO (nix - 8/10/22) Update crd status.
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
		diff        = control.diffFactory(kube.OrdinalAnnotation, kube.RevisionLabel, currentPVCs, wantPVCs)
	)

	var dataSource *corev1.TypedLocalObjectReference
	if len(diff.Creates()) > 0 {
		dataSource = control.autoDataSource(ctx, log, crd)
	}

	for _, pvc := range diff.Creates() {
		if pvc.Spec.DataSource == nil && pvc.Spec.DataSourceRef == nil {
			pvc.Spec.DataSource = dataSource
		}
		log.Info("Creating pvc", "pvcName", pvc.Name)
		if err := ctrl.SetControllerReference(crd, pvc, control.client.Scheme()); err != nil {
			return true, kube.TransientError(fmt.Errorf("set controller reference on pvc %q: %w", pvc.Name, err))
		}
		if err := control.client.Create(ctx, pvc); kube.IgnoreAlreadyExists(err) != nil {
			return true, kube.TransientError(fmt.Errorf("create pvc %q: %w", pvc.Name, err))
		}
	}

	var deletes int
	if !control.shouldRetain(crd) {
		for _, pvc := range diff.Deletes() {
			log.Info("Deleting pvc", "pvcName", pvc.Name)
			if err := control.client.Delete(ctx, pvc, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
				return true, kube.TransientError(fmt.Errorf("delete pvc %q: %w", pvc.Name, err))
			}
		}
		deletes = len(diff.Deletes())
	}

	if deletes+len(diff.Creates()) > 0 {
		// Scaling happens first; then updates. So requeue to handle updates after scaling finished.
		return true, nil
	}

	// PVCs have many immutable fields, so only update the storage size.
	for _, pvc := range diff.Updates() {
		// Only bound claims can be resized.
		found, ok := findMatchingResource(pvc, currentPVCs)
		if ok && found.Status.Phase != corev1.ClaimBound {
			continue
		}

		log.Info("Patching pvc", "pvcName", pvc.Name)
		patch := corev1.PersistentVolumeClaim{
			ObjectMeta: pvc.ObjectMeta,
			TypeMeta:   pvc.TypeMeta,
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: pvc.Spec.Resources,
			},
		}
		// It's safe to patch all PVCs at once. Pods must be restarted after resizing complete.
		if err := control.client.Patch(ctx, &patch, client.Merge); err != nil {
			// TODO (nix - 8/11/22) Update status with failures
			log.Error(err, "Patch failed", "pvcName", pvc.Name)
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

func (control PVCControl) autoDataSource(ctx context.Context, log logr.Logger, crd *cosmosv1.CosmosFullNode) *corev1.TypedLocalObjectReference {
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
		log.Error(err, "Failed to find VolumeSnapshot for AutoDataSource")
		return nil
	}

	return &corev1.TypedLocalObjectReference{
		APIGroup: ptr("snapshot.storage.k8s.io"),
		Kind:     "VolumeSnapshot",
		Name:     found.Name,
	}
}

func findMatchingResource[T client.Object](r T, list []T) (T, bool) {
	return lo.Find(list, func(other T) bool {
		return client.ObjectKeyFromObject(r) == client.ObjectKeyFromObject(other)
	})
}
