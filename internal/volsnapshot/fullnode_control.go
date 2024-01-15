package volsnapshot

import (
	"context"
	"fmt"
	"strings"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	cosmosalpha "github.com/bharvest-devops/cosmos-operator/api/v1alpha1"
	"github.com/bharvest-devops/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StatusSyncer interface {
	SyncUpdate(ctx context.Context, key client.ObjectKey, update func(status *cosmosv1.FullNodeStatus)) error
}

// FullNodeControl manages a ScheduledVolumeSnapshot's spec.fullNodeRef.
type FullNodeControl struct {
	client       client.Reader
	statusClient StatusSyncer
}

func NewFullNodeControl(statusClient StatusSyncer, client client.Reader) *FullNodeControl {
	return &FullNodeControl{client: client, statusClient: statusClient}
}

// SignalPodDeletion updates the LocalFullNodeRef's status to indicate it should delete the pod candidate.
// The pod is gracefully removed to ensure the highest data integrity while taking a VolumeSnapshot.
// Assumes crd's status.candidate is set, otherwise this method panics.
// Any error returned can be treated as transient and retried.
func (control FullNodeControl) SignalPodDeletion(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) error {
	key := control.sourceKey(crd)
	objKey := client.ObjectKey{Name: crd.Spec.FullNodeRef.Name, Namespace: crd.Namespace}
	return control.statusClient.SyncUpdate(ctx, objKey, func(status *cosmosv1.FullNodeStatus) {
		if status.ScheduledSnapshotStatus == nil {
			status.ScheduledSnapshotStatus = make(map[string]cosmosv1.FullNodeSnapshotStatus)
		}
		status.ScheduledSnapshotStatus[key] = cosmosv1.FullNodeSnapshotStatus{PodCandidate: crd.Status.Candidate.PodName}
	})
}

// SignalPodRestoration updates the LocalFullNodeRef's status to indicate it should recreate the pod candidate.
// Any error returned can be treated as transient and retried.
func (control FullNodeControl) SignalPodRestoration(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) error {
	key := control.sourceKey(crd)
	objKey := client.ObjectKey{Name: crd.Spec.FullNodeRef.Name, Namespace: crd.Namespace}
	return control.statusClient.SyncUpdate(ctx, objKey, func(status *cosmosv1.FullNodeStatus) {
		delete(status.ScheduledSnapshotStatus, key)
	})
}

// ConfirmPodRestoration verifies the pod has been restored.
func (control FullNodeControl) ConfirmPodRestoration(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) error {
	var (
		fullnode cosmosv1.CosmosFullNode
		getKey   = client.ObjectKey{Name: crd.Spec.FullNodeRef.Name, Namespace: crd.Namespace}
	)

	if err := control.client.Get(ctx, getKey, &fullnode); err != nil {
		return fmt.Errorf("get CosmosFullNode: %w", err)
	}

	if _, exists := fullnode.Status.ScheduledSnapshotStatus[control.sourceKey(crd)]; exists {
		return fmt.Errorf("pod %s not restored yet", crd.Status.Candidate.PodName)
	}

	return nil
}

// ConfirmPodDeletion returns a nil error if the pod is deleted.
// Any non-nil error is transient, including if the pod has not been deleted yet.
// Assumes crd's status.candidate is set, otherwise this method panics.
func (control FullNodeControl) ConfirmPodDeletion(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) error {
	var pods corev1.PodList
	if err := control.client.List(ctx, &pods,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Spec.FullNodeRef.Name},
	); err != nil {
		return fmt.Errorf("list pods: %w", err)
	}
	for _, pod := range pods.Items {
		if pod.Name == crd.Status.Candidate.PodName {
			return fmt.Errorf("pod %s not deleted yet", pod.Name)
		}
	}
	return nil
}

func (control FullNodeControl) sourceKey(crd *cosmosalpha.ScheduledVolumeSnapshot) string {
	key := strings.Join([]string{crd.Namespace, crd.Name, cosmosalpha.GroupVersion.Version, cosmosalpha.GroupVersion.Group}, ".")
	// Remove all slashes because key is used in JSONPatch where slash "/" is a reserved character.
	return strings.ReplaceAll(key, "/", "")
}
