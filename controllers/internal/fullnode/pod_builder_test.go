package fullnode

import (
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPodBuilder(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := cosmosv1.AddToScheme(scheme); err != nil {
		panic(err)
	}

	t.Parallel()

	crd := cosmosv1.CosmosFullNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "osmosis",
			Namespace: "test",
		},
		Spec: cosmosv1.CosmosFullNodeSpec{
			Image: "busybox:latest",
		},
	}

	t.Run("happy path", func(t *testing.T) {
		builder := NewPodBuilder(&crd)
		pod, err := builder.WithOrdinal(5).Build()
		require.NoError(t, err)

		require.Equal(t, "Pod", pod.Kind)
		require.Equal(t, "v1", pod.APIVersion)

		require.Equal(t, "test", pod.Namespace)
		require.Equal(t, "osmosis-5", pod.Name)
		require.Empty(t, pod.Annotations) // TODO: expose prom metrics
		wantLabels := map[string]string{
			"cosmosfullnode.cosmos.strange.love/chain-name":  "osmosis",
			"cosmosfullnode.cosmos.strange.love/pod-ordinal": "5",
		}
		require.Equal(t, wantLabels, pod.Labels)
		require.EqualValues(t, 30, *pod.Spec.TerminationGracePeriodSeconds)

		require.Len(t, pod.Spec.Containers, 1)
		c := pod.Spec.Containers[0]
		require.Equal(t, "osmosis", c.Name)
		require.Equal(t, "busybox:latest", c.Image)
		require.Empty(t, c.ImagePullPolicy)

		pod, err = builder.WithOwner(scheme).Build()
		require.NoError(t, err)

		require.Len(t, pod.OwnerReferences, 1)
		owner := pod.OwnerReferences[0]
		require.True(t, *owner.Controller)
		require.Equal(t, "CosmosFullNode", owner.Kind)
		require.Equal(t, "cosmos.strange.love/v1", owner.APIVersion)

		// Test we don't share or leak data per invocation.
		pod, err = builder.Build()
		require.NoError(t, err)
		require.Empty(t, pod.Name)

		pod, err = builder.WithOrdinal(123).Build()
		require.NoError(t, err)
		require.Equal(t, "osmosis-123", pod.Name)
	})

	t.Run("errors", func(t *testing.T) {
		invalidScheme := &runtime.Scheme{}
		builder := NewPodBuilder(&crd)
		_, err := builder.WithOwner(invalidScheme).Build()
		require.Error(t, err)

		_, err = builder.Build()
		require.NoError(t, err)
	})
}
