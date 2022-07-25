package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// PodState creates the final state of pods given the crd.
func PodState(crd *cosmosv1.CosmosFullNode) []*corev1.Pod {
	var (
		builder = NewPodBuilder(crd)
		pods    = make([]*corev1.Pod, crd.Spec.Replicas)
	)
	for i := int32(0); i < crd.Spec.Replicas; i++ {
		pod, err := builder.WithOrdinal(i).Build()
		if err != nil {
			// In this instance, there's nothing the builder should do to cause an error.
			panic(err)
		}
		pods[i] = pod
	}
	return pods
}
