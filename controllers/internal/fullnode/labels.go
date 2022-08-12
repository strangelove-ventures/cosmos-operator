package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	revisionLabel = "cosmosfullnode.cosmos.strange.love/resource-revision"
)

// SelectorLabels returns the labels used in selector operations.
func SelectorLabels(crd *cosmosv1.CosmosFullNode) client.MatchingLabels {
	return map[string]string{kube.NameLabel: appName(crd)}
}
