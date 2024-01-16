package fullnode

import (
	"errors"
	"fmt"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	"github.com/bharvest-devops/cosmos-operator/internal/kube"
)

const (
	networkLabel = "cosmos.b.harvest/network"
	typeLabel    = "cosmos.b.harvest/type"
)

// kv is a list of extra kv pairs to add to the labels. Must be even.
func defaultLabels(crd *cosmosv1.CosmosFullNode, kvPairs ...string) map[string]string {
	if len(kvPairs)%2 != 0 {
		panic(errors.New("key/value pairs must be even"))
	}
	nodeType := cosmosv1.FullNode
	if crd.Spec.Type != "" {
		nodeType = crd.Spec.Type
	}
	labels := map[string]string{
		kube.ControllerLabel: "cosmos-operator",
		kube.ComponentLabel:  cosmosv1.CosmosFullNodeController,
		kube.NameLabel:       appName(crd),
		kube.VersionLabel:    kube.ParseImageVersion(crd.Spec.PodTemplate.Image),
		networkLabel:         crd.Spec.ChainSpec.Network,
		typeLabel:            string(nodeType),
	}
	for i := 0; i < len(kvPairs); i += 2 {
		labels[kvPairs[i]] = kvPairs[i+1]
	}
	return labels
}

func appName(crd *cosmosv1.CosmosFullNode) string {
	return kube.ToName(crd.Name)
}

func instanceName(crd *cosmosv1.CosmosFullNode, ordinal int32) string {
	return kube.ToName(fmt.Sprintf("%s-%d", appName(crd), ordinal))
}

// Conditionally add custom labels or annotations, preserving key/values already set on 'into'.
// 'into' must not be nil.
func preserveMergeInto(into map[string]string, other map[string]string) {
	for k, v := range other {
		_, ok := into[k]
		if !ok {
			into[k] = v
		}
	}
}
