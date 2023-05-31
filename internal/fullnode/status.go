package fullnode

import (
	"context"

	"github.com/samber/lo"
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
) cosmosv1.SyncInfoStatus {
	var status cosmosv1.SyncInfoStatus

	coll := collector.Collect(ctx, client.ObjectKeyFromObject(crd))

	status.Pods = lo.Map(coll, func(item cosmos.StatusItem, _ int) cosmosv1.SyncInfoPodStatus {
		var stat cosmosv1.SyncInfoPodStatus
		stat.Pod = item.GetPod().Name
		stat.Timestamp = metav1.NewTime(item.Timestamp())
		comet, err := item.GetStatus()
		if err != nil {
			stat.Error = ptr(err.Error())
			return stat
		}
		stat.Height = ptr(comet.LatestBlockHeight())
		stat.InSync = ptr(!comet.Result.SyncInfo.CatchingUp)
		return stat
	})

	return status
}
