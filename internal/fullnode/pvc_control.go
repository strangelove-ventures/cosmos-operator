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
	"k8s.io/apimachinery/pkg/api/resource"
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

type PVCStatusChanges struct {
	Deleted []string
}

// Reconcile is the control loop for PVCs. The bool return value, if true, indicates the controller should requeue
// the request.
func (control PVCControl) Reconcile(ctx context.Context, reporter kube.Reporter, crd *cosmosv1.CosmosFullNode, pvcStatusChanges *PVCStatusChanges) (bool, kube.ReconcileError) {
	// Find any existing pvcs for this CRD.
	var vols corev1.PersistentVolumeClaimList
	if err := control.client.List(ctx, &vols,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
	); err != nil {
		return false, kube.TransientError(fmt.Errorf("list existing pvcs: %w", err))
	}

	var currentPVCs = ptrSlice(vols.Items)

	dataSources := make(map[int32]*dataSource)
	if len(currentPVCs) < int(crd.Spec.Replicas) {
		for i := int32(0); i < crd.Spec.Replicas; i++ {
			name := pvcName(crd, i)
			found := false
			for _, pvc := range currentPVCs {
				if pvc.Name == name {
					found = true
					break
				}
			}
			if !found {
				ds := control.findDataSource(ctx, reporter, crd, i)
				if ds == nil {
					ds = &dataSource{
						size: crd.Spec.VolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage],
					}
				}
				dataSources[i] = ds
			}
		}
	}

	var (
		wantPVCs = BuildPVCs(crd, dataSources, currentPVCs)
		diffed   = diff.New(currentPVCs, wantPVCs)
	)

	for _, pvc := range diffed.Creates() {
		size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]

		reporter.Info(
			"Creating pvc",
			"name", pvc.Name,
			"size", size.String(),
		)
		if err := ctrl.SetControllerReference(crd, pvc, control.client.Scheme()); err != nil {
			return true, kube.TransientError(fmt.Errorf("set controller reference on pvc %q: %w", pvc.Name, err))
		}
		if err := control.client.Create(ctx, pvc); kube.IgnoreAlreadyExists(err) != nil {
			return true, kube.TransientError(fmt.Errorf("create pvc %q: %w", pvc.Name, err))
		}
		pvcStatusChanges.Deleted = append(pvcStatusChanges.Deleted, pvc.Name)
	}

	var deletes int
	if !control.shouldRetain(crd) {
		for _, pvc := range diffed.Deletes() {
			reporter.Info("Deleting pvc", "name", pvc.Name)
			if err := control.client.Delete(ctx, pvc, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
				return true, kube.TransientError(fmt.Errorf("delete pvc %q: %w", pvc.Name, err))
			}
			pvcStatusChanges.Deleted = append(pvcStatusChanges.Deleted, pvc.Name)
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
		size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		reporter.Info(
			"Patching pvc",
			"name", pvc.Name,
			"size", size.String(), // TODO remove expensive operation
		)
		patch := corev1.PersistentVolumeClaim{
			ObjectMeta: pvc.ObjectMeta,
			TypeMeta:   pvc.TypeMeta,
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: pvc.Spec.Resources,
			},
		}
		if err := control.client.Patch(ctx, &patch, client.Merge); err != nil {
			reporter.Error(err, "PVC patch failed", "name", pvc.Name)
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

type dataSource struct {
	ref *corev1.TypedLocalObjectReference

	size resource.Quantity
}

func (control PVCControl) findDataSource(ctx context.Context, reporter kube.Reporter, crd *cosmosv1.CosmosFullNode, ordinal int32) *dataSource {
	if override, ok := crd.Spec.InstanceOverrides[instanceName(crd, ordinal)]; ok {
		if overrideTpl := override.VolumeClaimTemplate; overrideTpl != nil {
			return control.findDataSourceWithPvcSpec(ctx, reporter, crd, *overrideTpl, ordinal)
		}
	}

	return control.findDataSourceWithPvcSpec(ctx, reporter, crd, crd.Spec.VolumeClaimTemplate, ordinal)
}

func (control PVCControl) findDataSourceWithPvcSpec(
	ctx context.Context,
	reporter kube.Reporter,
	crd *cosmosv1.CosmosFullNode,
	pvcSpec cosmosv1.PersistentVolumeClaimSpec,
	ordinal int32,
) *dataSource {
	if ds := pvcSpec.DataSource; ds != nil {
		if ds.Kind == "VolumeSnapshot" && ds.APIGroup != nil && *ds.APIGroup == "snapshot.storage.k8s.io" {
			var vs snapshotv1.VolumeSnapshot
			if err := control.client.Get(ctx, client.ObjectKey{Namespace: crd.Namespace, Name: ds.Name}, &vs); err != nil {
				reporter.Error(err, "Failed to get VolumeSnapshot for DataSource")
				reporter.RecordError("DataSourceGetSnapshot", err)
				return nil
			}
			return &dataSource{
				ref:  ds,
				size: *vs.Status.RestoreSize,
			}
		} else if ds.Kind == "PersistentVolumeClaim" && (ds.APIGroup == nil || *ds.APIGroup == "") {
			var pvc corev1.PersistentVolumeClaim
			if err := control.client.Get(ctx, client.ObjectKey{Namespace: crd.Namespace, Name: ds.Name}, &pvc); err != nil {
				reporter.Error(err, "Failed to get PersistentVolumeClaim for DataSource")
				reporter.RecordError("DataSourceGetPVC", err)
				return nil
			}
			return &dataSource{
				ref:  ds,
				size: pvc.Status.Capacity["storage"],
			}
		} else {
			err := fmt.Errorf("unsupported DataSource %s", ds.Kind)
			reporter.Error(err, "Unsupported DataSource")
			reporter.RecordError("DataSourceUnsupported", err)
			return nil
		}
	}
	spec := pvcSpec.AutoDataSource
	if spec == nil {
		return nil
	}
	selector := spec.VolumeSnapshotSelector
	if len(selector) == 0 {
		return nil
	}
	if spec.MatchInstance {
		selector[kube.InstanceLabel] = instanceName(crd, ordinal)
	}
	found, err := control.recentVolumeSnapshot(ctx, control.client, crd.Namespace, selector)
	if err != nil {
		reporter.Error(err, "Failed to find VolumeSnapshot for AutoDataSource")
		reporter.RecordError("AutoDataSourceFindSnapshot", err)
		return nil
	}

	reporter.RecordInfo("AutoDataSource", "Using recent VolumeSnapshot for PVC data source")
	return &dataSource{
		ref: &corev1.TypedLocalObjectReference{
			APIGroup: ptr("snapshot.storage.k8s.io"),
			Kind:     "VolumeSnapshot",
			Name:     found.Name,
		},
		size: *found.Status.RestoreSize,
	}
}
