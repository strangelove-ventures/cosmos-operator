package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// BuildPods creates the final state of pods given the crd.
func BuildPods(crd *cosmosv1.CosmosFullNode) []*corev1.Pod {
	var (
		builder   = NewPodBuilder(crd)
		overrides = crd.Spec.InstanceOverrides
		pods      []*corev1.Pod
	)
	for i := int32(0); i < crd.Spec.Replicas; i++ {
		pod := builder.WithOrdinal(i).Build()
		if disable := overrides[pod.Name].DisableStrategy; disable != nil {
			continue
		}
		pods = append(pods, pod)
	}
	return pods
}
