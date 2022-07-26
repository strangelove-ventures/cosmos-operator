package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// OrdinalLabel denotes the resource's ordinal position. E.g. 0, 1, 2, 3
	// The label value must only be an integer.
	OrdinalLabel = "cosmosfullnode.cosmos.strange.love/ordinal"

	chainLabel = "cosmosfullnode.cosmos.strange.love/chain-name"
)

// SelectorLabels returns the labels used in selector operations.
func SelectorLabels(crd *cosmosv1.CosmosFullNode) client.MatchingLabels {
	return map[string]string{chainLabel: crd.Name}
}
