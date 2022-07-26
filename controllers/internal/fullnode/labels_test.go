package fullnode

import (
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSelectorLabels(t *testing.T) {
	t.Parallel()

	crd := &cosmosv1.CosmosFullNode{}
	crd.Name = "cool-chain"

	got := SelectorLabels(crd)
	require.Equal(t, client.MatchingLabels{"cosmosfullnode.cosmos.strange.love/chain-name": "cool-chain"}, got)
}
