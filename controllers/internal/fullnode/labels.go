package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	chainLabel   = "cosmosfullnode.cosmos.strange.love/chain-name"
	ordinalLabel = "cosmosfullnode.cosmos.strange.love/pod-ordinal"
)

// SelectorLabels returns the labels used in selector operations.
func SelectorLabels(crd *cosmosv1.CosmosFullNode) client.MatchingLabels {
	return map[string]string{chainLabel: crd.Name}
}
