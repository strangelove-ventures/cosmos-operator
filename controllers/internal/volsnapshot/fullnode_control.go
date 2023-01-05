package volsnapshot

import (
	"context"
	"fmt"
	"strings"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Patcher patches the status subresource of a CosmosFullNode.
type Patcher interface {
	Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error
}

// FullNodeControl manages a ScheduledVolumeSnapshot's spec.fullNodeRef.
type FullNodeControl struct {
	statusClient Patcher
}

func NewFullNodeControl(statusClient Patcher) *FullNodeControl {
	return &FullNodeControl{statusClient: statusClient}
}

// SignalPodDeletion updates the FullNodeRef's status to indicate it should delete the pod candidate.
// The pod is gracefully removed to ensure the highest data integrity while taking a VolumeSnapshot.
// Assumes crd's status.candidate is set, otherwise this method panics.
// Any error returned can be treated as transient and retried.
func (control FullNodeControl) SignalPodDeletion(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) error {
	var fn cosmosv1.CosmosFullNode
	fn.Name = crd.Spec.FullNodeRef.Name
	fn.Namespace = crd.Spec.FullNodeRef.Namespace
	fn.Status.ScheduledSnapshotStatus = make(map[string]cosmosv1.FullNodeSnapshotStatus)

	key := control.sourceKey(crd)
	fn.Status.ScheduledSnapshotStatus[key] = cosmosv1.FullNodeSnapshotStatus{
		PodCandidate: crd.Status.Candidate.PodName,
	}
	return control.statusClient.Patch(ctx, &fn, client.Merge)
}

// SignalPodRestoration updates the FullNodeRef's status to indicate it should recreate the pod candidate.
// Any error returned can be treated as transient and retried.
func (control FullNodeControl) SignalPodRestoration(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) error {
	var fn cosmosv1.CosmosFullNode
	fn.Name = crd.Spec.FullNodeRef.Name
	fn.Namespace = crd.Spec.FullNodeRef.Namespace
	key := control.sourceKey(crd)
	raw := client.RawPatch(types.JSONPatchType, []byte(fmt.Sprintf(`[{"op":"remove","path":"/status/scheduledSnapshotStatus/%s"}]`, key)))
	return control.statusClient.Patch(ctx, &fn, raw)
}

func (control FullNodeControl) sourceKey(crd *cosmosalpha.ScheduledVolumeSnapshot) string {
	key := strings.Join([]string{crd.Namespace, crd.Name, cosmosalpha.GroupVersion.Version, cosmosalpha.GroupVersion.Group}, ".")
	// Remove all slashes because key is used in JSONPatch where slash "/" is a reserved character.
	return strings.ReplaceAll(key, "/", "")
}
