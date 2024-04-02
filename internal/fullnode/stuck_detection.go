package fullnode

import (
	"context"
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type StuckPodDetection struct {
	available      func(pods []*corev1.Pod, minReady time.Duration, now time.Time) []*corev1.Pod
	collector      StatusCollector
	computeRollout func(maxUnavail *intstr.IntOrString, desired, ready int) int
}

// StuckPods returns pods that are stuck on a block height due to a cometbft issue that manifests on sentries using horcrux.
func (d StuckPodDetection) StuckPods(ctx context.Context, crd *cosmosv1.CosmosFullNode) []*corev1.Pod {
	//TODO
	return nil
}
