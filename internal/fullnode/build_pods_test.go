package fullnode

import (
	"fmt"
	"strconv"
	"testing"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	"github.com/bharvest-devops/cosmos-operator/internal/diff"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestBuildPods(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		crd := &cosmosv1.CosmosFullNode{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "agoric",
				Namespace: "test",
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
		appConfig := cosmosv1.SDKAppConfig{}
		crd.Spec.ChainSpec.CosmosSDK = &appConfig

		cksums := make(ConfigChecksums)
		for i := 0; i < int(crd.Spec.Replicas); i++ {
			cksums[client.ObjectKey{Namespace: crd.Namespace, Name: fmt.Sprintf("agoric-%d", i)}] = strconv.Itoa(i)
		}

		pods, err := BuildPods(crd, cksums)
		require.NoError(t, err)
		require.Equal(t, 5, len(pods))

		for i, r := range pods {
			require.Equal(t, int64(i), r.Ordinal(), i)
			require.NotEmpty(t, r.Revision(), i)
			require.Equal(t, strconv.Itoa(i), r.Object().Annotations["cosmos.bharvest/config-checksum"])
		}

		want := lo.Map([]int{0, 1, 2, 3, 4}, func(_ int, i int) string {
			return fmt.Sprintf("agoric-%d", i)
		})
		got := lo.Map(pods, func(pod diff.Resource[*corev1.Pod], _ int) string { return pod.Object().Name })
		require.Equal(t, want, got)

		pod, err := NewPodBuilder(crd).WithOrdinal(0).Build()
		require.NoError(t, err)
		require.Equal(t, pod.Spec, pods[0].Object().Spec)
	})

	t.Run("instance overrides", func(t *testing.T) {
		const (
			image         = "agoric:latest"
			overrideImage = "some_image:custom"
			overridePod   = "agoric-5"
		)

		cometConfig := cosmosv1.CometConfig{}
		appConfig := cosmosv1.SDKAppConfig{}
		crd := &cosmosv1.CosmosFullNode{
			ObjectMeta: metav1.ObjectMeta{
				Name: "agoric",
			},
			Spec: cosmosv1.FullNodeSpec{
				Replicas: 6,
				PodTemplate: cosmosv1.PodSpec{
					Image: image,
				},
				ChainSpec: cosmosv1.ChainSpec{
					Comet:     &cometConfig,
					CosmosSDK: &appConfig,
				},
				InstanceOverrides: map[string]cosmosv1.InstanceOverridesSpec{
					"agoric-2":  {DisableStrategy: ptr(cosmosv1.DisablePod)},
					"agoric-4":  {DisableStrategy: ptr(cosmosv1.DisableAll)},
					overridePod: {Image: overrideImage},
				},
			},
		}

		pods, err := BuildPods(crd, nil)
		require.NoError(t, err)
		require.Equal(t, 4, len(pods))

		want := lo.Map([]int{0, 1, 3, 5}, func(i int, _ int) string {
			return fmt.Sprintf("agoric-%d", i)
		})
		got := lo.Map(pods, func(pod diff.Resource[*corev1.Pod], _ int) string { return pod.Object().Name })
		require.Equal(t, want, got)
		for _, pod := range pods {
			image := pod.Object().Spec.Containers[0].Image
			if pod.Object().Name == overridePod {
				require.Equal(t, overrideImage, image)
			} else {
				require.Equal(t, image, image)
			}
		}
	})

	t.Run("scheduled volume snapshot pod candidate", func(t *testing.T) {
		cometConfig := cosmosv1.CometConfig{}
		appConfig := cosmosv1.SDKAppConfig{}
		crd := &cosmosv1.CosmosFullNode{
			ObjectMeta: metav1.ObjectMeta{
				Name: "agoric",
			},
			Spec: cosmosv1.FullNodeSpec{
				Replicas: 6,
				ChainSpec: cosmosv1.ChainSpec{
					Comet:     &cometConfig,
					CosmosSDK: &appConfig,
				},
			},
			Status: cosmosv1.FullNodeStatus{
				ScheduledSnapshotStatus: map[string]cosmosv1.FullNodeSnapshotStatus{
					"some.scheduled.snapshot.1":       {PodCandidate: "agoric-1"},
					"some.scheduled.snapshot.2":       {PodCandidate: "agoric-2"},
					"some.scheduled.snapshot.ignored": {PodCandidate: "agoric-99"},
				},
			},
		}

		pods, err := BuildPods(crd, nil)
		require.NoError(t, err)
		require.Equal(t, 4, len(pods))

		want := lo.Map([]int{0, 3, 4, 5}, func(i int, _ int) string {
			return fmt.Sprintf("agoric-%d", i)
		})
		got := lo.Map(pods, func(pod diff.Resource[*corev1.Pod], _ int) string { return pod.Object().Name })
		require.Equal(t, want, got)
	})
}
