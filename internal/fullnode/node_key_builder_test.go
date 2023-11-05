package fullnode

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/strangelove-ventures/cosmos-operator/internal/test"

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

		for i, s := range secrets {
			require.Equal(t, int64(i), s.Ordinal())
			require.NotEmpty(t, s.Revision())
			got := s.Object()
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
				"cosmos.strange.love/type":     "FullNode",
			}
			require.Equal(t, wantLabels, got.Labels)

			require.Empty(t, got.Annotations)

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
		const namespace = "test-namespace"
		var crd cosmosv1.CosmosFullNode
		crd.Namespace = namespace
		crd.Name = "juno"
		crd.Spec.Replicas = 3

		var existing corev1.Secret
		existing.Name = "juno-node-key-0"
		existing.Namespace = namespace
		existing.Annotations = map[string]string{"foo": "bar"}
		existing.Data = map[string][]byte{"node_key.json": []byte("existing")}

		got, err := BuildNodeKeySecrets([]*corev1.Secret{&existing}, &crd)
		require.NoError(t, err)
		require.Equal(t, 3, len(got))

		nodeKey := got[0].Object().Data["node_key.json"]
		require.Equal(t, "existing", string(nodeKey))

		require.Empty(t, got[0].Object().Annotations)
	})

	t.Run("zero replicas", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		secrets, err := BuildNodeKeySecrets(nil, &crd)
		require.NoError(t, err)
		require.Empty(t, secrets)
	})

	test.HasTypeLabel(t, func(crd cosmosv1.CosmosFullNode) []map[string]string {
		secrets, _ := BuildNodeKeySecrets(nil, &crd)
		labels := make([]map[string]string, 0)
		for _, secret := range secrets {
			labels = append(labels, secret.Object().Labels)
		}
		return labels
	})
}
