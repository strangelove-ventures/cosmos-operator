package fullnode

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestBuildNodeKeySecrets(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Namespace = "test-namespace"
		crd.Name = "juno"
		crd.Spec.Replicas = 3
		crd.Spec.ChainSpec.Network = "mainnet"
		crd.Spec.PodTemplate.Image = "ghcr.io/juno:v1.2.3"

		secrets, err := BuildNodeKeySecrets(nil, &crd)
		require.NoError(t, err)
		require.Len(t, secrets, 3)

		for i, got := range secrets {
			require.Equal(t, crd.Namespace, got.Namespace)
			require.Equal(t, fmt.Sprintf("juno-node-key-%d", i), got.Name)
			require.Equal(t, "Secret", got.Kind)
			require.Equal(t, "v1", got.APIVersion)

			wantLabels := map[string]string{
				"app.kubernetes.io/created-by": "cosmos-operator",
				"app.kubernetes.io/component":  "CosmosFullNode",
				"app.kubernetes.io/name":       "juno",
				"app.kubernetes.io/instance":   fmt.Sprintf("juno-%d", i),
				"app.kubernetes.io/version":    "v1.2.3",
				"cosmos.strange.love/network":  "mainnet",
			}
			require.Equal(t, wantLabels, got.Labels)

			wantAnnotations := map[string]string{
				"app.kubernetes.io/ordinal": strconv.Itoa(i),
			}
			require.Equal(t, wantAnnotations, got.Annotations)

			require.True(t, *got.Immutable)
			require.Equal(t, corev1.SecretTypeOpaque, got.Type)

			nodeKey := got.Data["node_key.json"]
			require.NotEmpty(t, nodeKey)

			var gotJSON map[string]map[string]string
			err = json.Unmarshal(nodeKey, &gotJSON)
			require.NoError(t, err)
			require.Equal(t, gotJSON["priv_key"]["type"], "tendermint/PrivKeyEd25519")
			require.NotEmpty(t, gotJSON["priv_key"]["value"])
		}
	})

	t.Run("with existing", func(t *testing.T) {
		t.Fatal("TODO")
	})

	t.Run("zero replicas", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		secrets, err := BuildNodeKeySecrets(nil, &crd)
		require.NoError(t, err)
		require.Empty(t, secrets)
	})
}
