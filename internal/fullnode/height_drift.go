package fullnode

import (
	"context"
	"time"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/cosmos"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DriftDetection struct {
	available      func(pods []*corev1.Pod, minReady time.Duration, now time.Time) []*corev1.Pod
	collector      StatusCollector
	computeRollout func(maxUnavail *intstr.IntOrString, desired, ready int) int
}

func NewDriftDetection(collector StatusCollector) DriftDetection {
	return DriftDetection{
		available:      kube.AvailablePods,
		collector:      collector,
		computeRollout: kube.ComputeRollout,
	}
}

func (d DriftDetection) LaggingPods(ctx context.Context, crd *cosmosv1.CosmosFullNode) []*corev1.Pod {
	coll := d.collector.Collect(ctx, client.ObjectKeyFromObject(crd)).Synced()
	thresh := uint64(crd.Spec.SelfHeal.HeightDriftMitigation.Threshold)
	maxHeight := lo.MaxBy(coll, func(a cosmos.StatusItem, b cosmos.StatusItem) bool {
		return a.Status.LatestBlockHeight() > b.Status.LatestBlockHeight()
	}).Status.LatestBlockHeight()
	lagging := lo.FilterMap(coll, func(item cosmos.StatusItem, _ int) (*corev1.Pod, bool) {
		inSync := !item.Status.Result.SyncInfo.CatchingUp
		isLagging := maxHeight-item.Status.LatestBlockHeight() >= thresh
		return item.GetPod(), inSync && isLagging
	})
	avail := d.available(lagging, 5*time.Second, time.Now())
	rollout := d.computeRollout(crd.Spec.RolloutStrategy.MaxUnavailable, int(crd.Spec.Replicas), len(avail))
	return lo.Slice(avail, 0, rollout)
}
