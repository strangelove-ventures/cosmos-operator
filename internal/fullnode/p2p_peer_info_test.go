package fullnode

import (
	"testing"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestBuildPeerInfo(t *testing.T) {
	t.Parallel()

	const (
		namespace = "strangelove"
		nodeKey   = `{"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"HBX8VFQ4OdWfOwIOR7jj0af8mVHik5iGW9o1xnn4vRltk1HmwQS2LLGrMPVS2LIUO9BUqmZ1Pjt+qM8x0ibHxQ=="}}`
	)

	t.Run("happy path", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Name = "agoric"
		crd.Namespace = namespace
		crd.Spec.Replicas = 3
		res, err := BuildNodeKeySecrets(nil, &crd)
		require.NoError(t, err)
		secrets := lo.Map(res, func(r diff.Resource[*corev1.Secret], _ int) *corev1.Secret { return r.Object() })

		secrets[0].Data[nodeKeyFile] = []byte(nodeKey)
		gotInfo, err := BuildPeerInfo(secrets, &crd)
		require.NoError(t, err)
		require.Len(t, gotInfo, 3)

		require.Len(t, lo.Uniq(lo.Map(lo.Values(gotInfo), func(v PeerInfo, _ int) string { return v.NodeID })), 3)
		require.Len(t, lo.Uniq(lo.Map(lo.Values(gotInfo), func(v PeerInfo, _ int) string { return v.PrivateAddress })), 3)

		got := gotInfo[client.ObjectKey{Name: "agoric-0", Namespace: namespace}]
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892", got.NodeID)
		require.Equal(t, "1e23ce0b20ae2377925537cc71d1529d723bb892@agoric-p2p-0.strangelove.svc.cluster.local:26656", got.PrivateAddress)

		got = gotInfo[client.ObjectKey{Name: "agoric-1", Namespace: namespace}]
		require.NotEmpty(t, got.NodeID)
		require.NotEmpty(t, got.PrivateAddress)

		got = gotInfo[client.ObjectKey{Name: "agoric-2", Namespace: namespace}]
		require.NotEmpty(t, got.NodeID)
		require.NotEmpty(t, got.PrivateAddress)
	})
}
