package volsnapshot

import (
	"context"
	"fmt"
	"time"

	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/fullnode"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PodFinder interface {
	SyncedPod(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error)
}

// VolumeSnapshotControl manages VolumeSnapshots
type VolumeSnapshotControl struct {
	client client.Reader
	finder PodFinder
}

func NewVolumeSnapshotControl(client client.Reader, finder PodFinder) *VolumeSnapshotControl {
	return &VolumeSnapshotControl{client: client, finder: finder}
}

// Candidate is a target instance of a CosmosFullNode from which to make a snapshot.
type Candidate struct {
	PodName string
	PVCName string
}

// FindCandidate finds a suitable candidate for creating a volume snapshot.
// Any errors returned can be treated as transient; worth a retry.
func (control VolumeSnapshotControl) FindCandidate(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) (Candidate, error) {
	var pods corev1.PodList
	if err := control.client.List(ctx, &pods,
		client.InNamespace(crd.Spec.SourceRef.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Spec.SourceRef.Name},
	); err != nil {
		return Candidate{}, err
	}

	avail := int32(len(kube.AvailablePods(ptrSlice(pods.Items), 0, time.Now())))
	minAvail := crd.Spec.MinAvailable
	if minAvail <= 0 {
		minAvail = 2
	}

	if avail < minAvail {
		return Candidate{}, fmt.Errorf("%d or more pods must be in a ready state to prevent downtime, found %d available", minAvail, avail)
	}

	pod, err := control.finder.SyncedPod(ctx, ptrSlice(pods.Items))
	if err != nil {
		return Candidate{}, err
	}

	return Candidate{PodName: pod.Name, PVCName: fullnode.PVCName(pod)}, nil
}
