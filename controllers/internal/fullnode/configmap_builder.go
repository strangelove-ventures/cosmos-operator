package fullnode

import (
	"bytes"
	_ "embed"
	"fmt"
	"net"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/peterbourgon/mergemap"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// BuildConfigMap creates a config map with configuration to be mounted as files into containers.
// Currently, the config.toml (for Tendermint) and app.toml (for the Cosmos SDK).
func BuildConfigMap(tendermint cosmosv1.CosmosTendermintConfig, app cosmosv1.CosmosAppConfig) (corev1.ConfigMap, error) {
	var cm corev1.ConfigMap
	dst := baseTendermint()
	mergemap.Merge(dst, decodeTendermint(tendermint))

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

func decodeTendermint(tendermint cosmosv1.CosmosTendermintConfig) decodedToml {
	base := make(decodedToml)
	if v := tendermint.LogLevel; v != nil {
		base["log_level"] = v
	}
	if v := tendermint.LogFormat; v != nil {
		base["log_format"] = v
	}

	p2p := decodedToml{
		"external_address":       net.JoinHostPort(tendermint.ExternalAddress, "26656"),
		"persistent_peers":       strings.Join(tendermint.PersistentPeers, ","),
		"seeds":                  strings.Join(tendermint.Seeds, ","),
		"max_num_inbound_peers":  tendermint.MaxInboundPeers,
		"max_num_outbound_peers": tendermint.MaxOutboundPeers,
	}
	base["p2p"] = p2p

	if v := tendermint.CorsAllowedOrigins; v != nil {
		base["rpc"] = decodedToml{"cors_allowed_origins": v}
	}

	return base
}
