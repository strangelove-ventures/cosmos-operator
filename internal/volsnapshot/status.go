package volsnapshot

import (
	"time"

	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResetStatus resets the CRD's status to appropriate values for the start of a reconcile loop.
func ResetStatus(crd *cosmosalpha.ScheduledVolumeSnapshot) {
	crd.Status.ObservedGeneration = crd.Generation
	crd.Status.StatusMessage = nil
	if crd.Status.CreatedAt.IsZero() {
		crd.Status.CreatedAt = metav1.NewTime(time.Now())
	}
	switch {
	case crd.Spec.Suspend:
		// Restore any temporarily deleted pod and suspend
		crd.Status.Phase = cosmosalpha.SnapshotPhaseRestorePod
	case !crd.Spec.Suspend && crd.Status.Phase == cosmosalpha.SnapshotPhaseSuspended:
		// If user reactivates, reset to beginning.
		crd.Status.Phase = cosmosalpha.SnapshotPhaseWaitingForNext
	case crd.Status.Phase == "":
		// CRD was just created.
		crd.Status.Phase = cosmosalpha.SnapshotPhaseWaitingForNext
	}
}
