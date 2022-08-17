package fullnode

import (
	"errors"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	networkLabel = "cosmos.strange.love/network"
)

// SelectorLabels returns the labels used in selector operations.
func SelectorLabels(crd *cosmosv1.CosmosFullNode) client.MatchingLabels {
	return map[string]string{kube.NameLabel: appName(crd)}
}

// kv is a list of extra kv pairs to add to the labels. Must be even.
func defaultLabels(crd *cosmosv1.CosmosFullNode, kvPairs ...string) map[string]string {
	if len(kvPairs)%2 != 0 {
		panic(errors.New("key/value pairs must be even"))
	}
	labels := map[string]string{
		kube.ControllerLabel: "cosmosfullnode",
		kube.NameLabel:       appName(crd),
		kube.VersionLabel:    kube.ParseImageVersion(crd.Spec.PodTemplate.Image),
		networkLabel:         kube.ToLabelValue(crd.Spec.ChainConfig.Network),
	}
	for i := 0; i < len(kvPairs); i += 2 {
		labels[kvPairs[i]] = kvPairs[i+1]
	}
	return labels
}

func appName(crd *cosmosv1.CosmosFullNode) string {
	return kube.ToLabelValue(fmt.Sprintf("%s-%s-fullnode", crd.Name, crd.Spec.ChainConfig.Network))
}
