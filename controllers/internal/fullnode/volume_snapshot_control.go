package fullnode

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type VolumeSnapshotCandidate struct {
	podName      string
	pvcName      string
	needsRequeue bool
}

func (candidate VolumeSnapshotCandidate) PodName() string    { return candidate.podName }
func (candidate VolumeSnapshotCandidate) PVCName() string    { return candidate.pvcName }
func (candidate VolumeSnapshotCandidate) NeedsRequeue() bool { return candidate.needsRequeue }

func (candidate VolumeSnapshotCandidate) Valid() bool {
	return candidate.podName != "" && candidate.pvcName != ""
}

type SyncedPodFinder interface {
	SyncedPod(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error)
}

type VolumeSnapshotControl struct {
	client    Client
	podFinder SyncedPodFinder
}

func NewVolumeSnapshotControl(client Client, finder SyncedPodFinder) *VolumeSnapshotControl {
	return &VolumeSnapshotControl{
		client:    client,
		podFinder: finder,
	}
}

func (vc VolumeSnapshotControl) FindCandidate(ctx context.Context, crd *cosmosv1.CosmosFullNode, now time.Time) (VolumeSnapshotCandidate, kube.ReconcileError) {
	if crd.Spec.VolumeSnapshot == nil {
		return VolumeSnapshotCandidate{}, nil
	}

	sched, err := cron.ParseStandard(crd.Spec.VolumeSnapshot.Schedule)
	if err != nil {
		// Although this error cannot be solved without human intervention, mark as transient, so we don't
		// halt reconciling. Creating VolumeSnapshots is ancillary and not as important as keeping the
		// fullnode deployment healthy.
		// Ideally, validation happens upstream in an admission controller.

	}

	var pods corev1.PodList
	if err := vc.client.List(ctx, &pods,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
	); err != nil {
		return VolumeSnapshotCandidate{}, kube.TransientError(err)
	}

	// > 1 available, no-op and requeue

	pod, err := vc.podFinder.SyncedPod(ctx, ptrSlice(pods.Items))
	if err != nil {
		return VolumeSnapshotCandidate{}, kube.TransientError(err)
	}

	return VolumeSnapshotCandidate{
		podName: pod.Name,
		pvcName: kube.ToName(fmt.Sprintf("pvc-%s", pod.Name)),
	}, nil
}
