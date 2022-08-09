package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	chainLabel = "cosmosfullnode.cosmos.strange.love/chain-name"

	// Denotes the resource's revision typically using hex-encoded fnv hash. Used to detect resource changes for updates.
	revisionLabel = "cosmosfullnode.cosmos.strange.love/resource-revision"
)

// SelectorLabels returns the labels used in selector operations.
func SelectorLabels(crd *cosmosv1.CosmosFullNode) client.MatchingLabels {
	return map[string]string{chainLabel: crd.Name}
}
