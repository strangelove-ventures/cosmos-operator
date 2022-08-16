package fullnode

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/peterbourgon/mergemap"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildConfigMap creates a ConfigMap with configuration to be mounted as files into containers.
// Currently, the config.toml (for Tendermint) and app.toml (for the Cosmos SDK).
func BuildConfigMap(crd *cosmosv1.CosmosFullNode) (corev1.ConfigMap, error) {
	var (
		cm = corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName(crd),
				Namespace: crd.Namespace,
				Labels: map[string]string{
					kube.ControllerLabel: kube.ToLabelValue("CosmosFullNode"),
					kube.NameLabel:       appName(crd),
					kube.VersionLabel:    kube.ParseImageVersion(crd.Spec.PodTemplate.Image),
				},
			},
		}
		tendermint = crd.Spec.ChainConfig.Tendermint
		dst        = baseTendermint()
	)

	base := make(decodedToml)
	if v := crd.Spec.ChainConfig.LogLevel; v != nil {
		base["log_level"] = v
	}
	if v := crd.Spec.ChainConfig.LogFormat; v != nil {
		base["log_format"] = v
	}

	mergemap.Merge(dst, addTendermint(base, tendermint))

	if overrides := tendermint.TomlOverrides; overrides != nil {
		var decoded decodedToml
		_, err := toml.Decode(*overrides, &decoded)
		if err != nil {
			return cm, fmt.Errorf("invalid toml in overrides: %w", err)
		}
		mergemap.Merge(dst, decoded)
	}

	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(dst); err != nil {
		return cm, err
	}
	cm.Data = map[string]string{"config.toml": buf.String()}

	return cm, nil
}

func configMapName(crd *cosmosv1.CosmosFullNode) string {
	return kube.ToName(fmt.Sprintf("%s-fullnode-config", crd.Name))
}

type decodedToml = map[string]any

//go:embed tendermint_config.toml
var baseTendermintToml []byte

func baseTendermint() decodedToml {
	var data decodedToml
	if err := toml.Unmarshal(baseTendermintToml, &data); err != nil {
		panic(err)
	}
	return data
}

func addTendermint(base decodedToml, tendermint cosmosv1.CosmosTendermintConfig) decodedToml {
	p2p := decodedToml{
		"persistent_peers": tendermint.PersistentPeers,
		"seeds":            tendermint.Seeds,
	}
	if v := tendermint.MaxInboundPeers; v != nil {
		p2p["max_num_inbound_peers"] = tendermint.MaxInboundPeers
	}
	if v := tendermint.MaxOutboundPeers; v != nil {
		p2p["max_num_outbound_peers"] = tendermint.MaxOutboundPeers
	}
	base["p2p"] = p2p

	if v := tendermint.CorsAllowedOrigins; v != nil {
		base["rpc"] = decodedToml{"cors_allowed_origins": v}
	}

	return base
}
