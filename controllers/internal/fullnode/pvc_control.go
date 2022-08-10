package fullnode

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
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
	client      Client
	diffFactory func(ordinalAnnotationKey string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer
}

// NewPVCControl returns a valid PVCControl
func NewPVCControl(client Client) PVCControl {
	return PVCControl{
		client: client,
		diffFactory: func(ordinalAnnotationKey string, current, want []*corev1.PersistentVolumeClaim) pvcDiffer {
			return kube.NewDiff(ordinalAnnotationKey, current, want)
		},
	}
}

// Reconcile is the control loop for PVCs. The bool return value, if true, indicates the controller should requeue
// the request.
func (vc PVCControl) Reconcile(ctx context.Context, log logr.Logger, crd *cosmosv1.CosmosFullNode) (bool, kube.ReconcileError) {
	// TODO (nix - 8/10/22) Update crd status.
	// Find any existing pvcs for this CRD.
	var vols corev1.PersistentVolumeClaimList
	if err := vc.client.List(ctx, &vols,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
		SelectorLabels(crd),
	); err != nil {
		return false, kube.TransientError(fmt.Errorf("list existing pvcs: %w", err))
	}

	var (
		currentPVCs = ptrSlice(vols.Items)
		wantPVCs    = BuildPVCs(crd)
		diff        = vc.diffFactory(OrdinalAnnotation, currentPVCs, wantPVCs)
	)

	for _, pvc := range diff.Creates() {
		log.Info("Creating pvc", "pvcName", pvc.Name)
		if err := ctrl.SetControllerReference(crd, pvc, vc.client.Scheme()); err != nil {
			return true, kube.TransientError(fmt.Errorf("set controller reference on pvc %q: %w", pvc.Name, err))
		}
		if err := vc.client.Create(ctx, pvc); err != nil {
			return true, kube.TransientError(fmt.Errorf("create pvc %q: %w", pvc.Name, err))
		}
	}

	for _, pvc := range diff.Deletes() {
		log.Info("Deleting pvc", "pvcName", pvc.Name)
		if err := vc.client.Delete(ctx, pvc, client.PropagationPolicy(metav1.DeletePropagationForeground)); client.IgnoreNotFound(err) != nil {
			return true, kube.TransientError(fmt.Errorf("delete pvc %q: %w", pvc.Name, err))
		}
	}

	return false, nil
}
