package volsnapshot

import (
	"context"
	"fmt"
	"strings"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StatusPatcher patches the status subresource of a CosmosFullNode.
type StatusPatcher interface {
	Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error
}

// FullNodeControl manages a ScheduledVolumeSnapshot's spec.fullNodeRef.
type FullNodeControl struct {
	client       client.Reader
	statusClient StatusPatcher
}

func NewFullNodeControl(statusClient StatusPatcher, client client.Reader) *FullNodeControl {
	return &FullNodeControl{client: client, statusClient: statusClient}
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
// This method will error if the scheduledSnapshotStatus map key does not exist. You get an unhelpful error message from
// the k8s API: "The request is invalid: the server rejected our request due to an error in our request"
func (control FullNodeControl) SignalPodRestoration(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) error {
	var fn cosmosv1.CosmosFullNode
	fn.Name = crd.Spec.FullNodeRef.Name
	fn.Namespace = crd.Spec.FullNodeRef.Namespace
	key := control.sourceKey(crd)
	raw := client.RawPatch(types.JSONPatchType, []byte(fmt.Sprintf(`[{"op":"remove","path":"/status/scheduledSnapshotStatus/%s"}]`, key)))
	return control.statusClient.Patch(ctx, &fn, raw)
}

// ConfirmPodRestoration verifies the pod has been restored.
func (control FullNodeControl) ConfirmPodRestoration(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) error {
	var pods corev1.PodList
	if err := control.client.List(ctx, &pods,
		client.InNamespace(crd.Spec.FullNodeRef.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Spec.FullNodeRef.Name},
	); err != nil {
		return fmt.Errorf("list pods: %w", err)
	}
	for _, pod := range pods.Items {
		if pod.Name == crd.Status.Candidate.PodName {
			return nil
		}
	}
	return fmt.Errorf("pod %s not restored yet", crd.Status.Candidate.PodName)
}

// ConfirmPodDeletion returns a nil error if the pod is deleted.
// Any non-nil error is transient, including if the pod has not been deleted yet.
// Assumes crd's status.candidate is set, otherwise this method panics.
func (control FullNodeControl) ConfirmPodDeletion(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) error {
	var pods corev1.PodList
	if err := control.client.List(ctx, &pods,
		client.InNamespace(crd.Spec.FullNodeRef.Namespace),
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
