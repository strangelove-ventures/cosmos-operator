package fullnode

import (
	"context"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/cosmos"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResetStatus is used at the beginning of the reconcile loop.
// It resets the crd's status to a fresh state.
func ResetStatus(crd *cosmosv1.CosmosFullNode) {
	crd.Status.ObservedGeneration = crd.Generation
	crd.Status.Phase = cosmosv1.FullNodePhaseProgressing
	crd.Status.StatusMessage = nil
}

type StatusCollector interface {
	Collect(ctx context.Context, controller client.ObjectKey) cosmos.StatusCollection
}

// SyncInfoStatus returns the status of the full node's sync info.
func SyncInfoStatus(
	ctx context.Context,
	crd *cosmosv1.CosmosFullNode,
	collector StatusCollector,
) map[string]*cosmosv1.SyncInfoPodStatus {
	status := make(map[string]*cosmosv1.SyncInfoPodStatus, crd.Spec.Replicas)

	coll := collector.Collect(ctx, client.ObjectKeyFromObject(crd))

	for _, item := range coll {
		var stat cosmosv1.SyncInfoPodStatus
		podName := item.GetPod().Name
		stat.Timestamp = metav1.NewTime(item.Timestamp())
		comet, err := item.GetStatus()
		if err != nil {
			stat.Error = ptr(err.Error())
			status[podName] = &stat
			continue
		}
		stat.Height = ptr(comet.LatestBlockHeight())
		stat.InSync = ptr(!comet.Result.SyncInfo.CatchingUp)
		status[podName] = &stat
	}

	return status
}
