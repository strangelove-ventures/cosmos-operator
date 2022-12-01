package fullnode

import (
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResetStatus is used at the beginning of the reconcile loop.
// It resets the crd's status to a fresh state.
func ResetStatus(crd *cosmosv1.CosmosFullNode) {
	crd.Status.ObservedGeneration = crd.Generation
	crd.Status.Phase = cosmosv1.FullNodePhaseProgressing
	crd.Status.StatusMessage = nil

	if crd.Spec.VolumeSnapshot == nil {
		crd.Status.VolumeSnapshot = nil
		return
	}

	if crd.Status.VolumeSnapshot == nil {
		crd.Status.VolumeSnapshot = new(cosmosv1.VolumeSnapshotStatus)
	}

	if crd.Status.VolumeSnapshot.ActivatedAt.IsZero() {
		crd.Status.VolumeSnapshot.ActivatedAt = metav1.NewTime(time.Now())
	}
}
