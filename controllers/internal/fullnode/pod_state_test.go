package fullnode

import (
	"fmt"
	"testing"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodState(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		crd := &cosmosv1.CosmosFullNode{
			ObjectMeta: metav1.ObjectMeta{
				Name: "agoric",
			},
			Spec: cosmosv1.FullNodeSpec{
				Replicas:  5,
				ChainSpec: cosmosv1.ChainSpec{Network: "devnet"},
				PodTemplate: cosmosv1.PodSpec{
					Image: "busybox:latest",
				},
				InstanceOverrides: nil,
			},
		}

		pods := PodState(crd)
		require.Equal(t, 5, len(pods))

		want := lo.Map([]int{0, 1, 2, 3, 4}, func(_ int, i int) string {
			return fmt.Sprintf("agoric-%d", i)
		})
		got := lo.Map(pods, func(pod *corev1.Pod, _ int) string { return pod.Name })
		require.Equal(t, want, got)

		pod := NewPodBuilder(crd).WithOrdinal(0).Build()
		require.Equal(t, pod, pods[0])
	})

	t.Run("instance overrides", func(t *testing.T) {
		crd := &cosmosv1.CosmosFullNode{
			ObjectMeta: metav1.ObjectMeta{
				Name: "agoric",
			},
			Spec: cosmosv1.FullNodeSpec{
				Replicas: 6,
				InstanceOverrides: map[string]cosmosv1.InstanceOverridesSpec{
					"agoric-2": {DisableStrategy: ptr(cosmosv1.DisablePod)},
					"agoric-4": {DisableStrategy: ptr(cosmosv1.DisableAll)},
				},
			},
		}

		pods := PodState(crd)
		require.Equal(t, 4, len(pods))

		want := lo.Map([]int{0, 1, 3, 5}, func(i int, _ int) string {
			return fmt.Sprintf("agoric-%d", i)
		})
		got := lo.Map(pods, func(pod *corev1.Pod, _ int) string { return pod.Name })
		require.Equal(t, want, got)
	})
}
