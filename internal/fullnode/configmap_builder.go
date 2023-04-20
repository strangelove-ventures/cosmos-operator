package fullnode

import (
	"bytes"
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/peterbourgon/mergemap"
	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
)

const (
	configOverlayFile = "config-overlay.toml"
	appOverlayFile    = "app-overlay.toml"
)

// BuildConfigMaps creates a ConfigMap with configuration to be mounted as files into containers.
// Currently, the config.toml (for Tendermint) and app.toml (for the Cosmos SDK).
func BuildConfigMaps(crd *cosmosv1.CosmosFullNode, peers Peers) ([]diff.Resource[*corev1.ConfigMap], error) {
	var (
		buf = bufPool.Get().(*bytes.Buffer)
		cms = make([]diff.Resource[*corev1.ConfigMap], crd.Spec.Replicas)
	)
	defer bufPool.Put(buf)
	defer buf.Reset()

	for i := int32(0); i < crd.Spec.Replicas; i++ {
		data := make(map[string]string)
		instance := instanceName(crd, i)
		if err := addConfigToml(buf, data, crd, instance, peers); err != nil {
			return nil, err
		}
		buf.Reset()
		if err := addAppToml(buf, data, crd.Spec.ChainSpec.App); err != nil {
			return nil, err
		}
		buf.Reset()

		var cm corev1.ConfigMap
		cm.Name = instanceName(crd, i)
		cm.Namespace = crd.Namespace
		cm.Kind = "ConfigMap"
		cm.APIVersion = "v1"
		cm.Labels = defaultLabels(crd,
			kube.InstanceLabel, instanceName(crd, i),
		)
		cm.Data = data
		kube.NormalizeMetadata(&cm.ObjectMeta)
		cms[i] = diff.Adapt(&cm, i)
	}

	return cms, nil
}

type decodedToml = map[string]any

//go:embed toml/tendermint_default_config.toml
var defaultTendermintToml []byte

func defaultTendermint() decodedToml {
	var data decodedToml
	if err := toml.Unmarshal(defaultTendermintToml, &data); err != nil {
		panic(err)
	}
	return data
}

//go:embed toml/app_default_config.toml
var defaultAppToml []byte

func defaultApp() decodedToml {
	var data decodedToml
	if err := toml.Unmarshal(defaultAppToml, &data); err != nil {
		panic(err)
	}
	return data
}

func addConfigToml(buf *bytes.Buffer, cmData map[string]string, crd *cosmosv1.CosmosFullNode, instance string, peers Peers) error {
	var (
		spec = crd.Spec.ChainSpec
		base = make(decodedToml)
	)

	if crd.Spec.Type == cosmosv1.FullNodeSentry {
		base["priv_validator_laddr"] = fmt.Sprintf("tcp://0.0.0.0:%d", privvalPort)
		// Disable indexing for sentries; they should not serve queries.
		base["tx_index"] = map[string]string{"indexer": "null"}
	}
	if v := spec.LogLevel; v != nil {
		base["log_level"] = v
	}
	if v := spec.LogFormat; v != nil {
		base["log_format"] = v
	}

	privatePeers := peers.Except(instance, crd.Namespace)
	privatePeerStr := commaDelimited(privatePeers.AllPrivate()...)
	tendermint := spec.Tendermint
	p2p := decodedToml{
		"persistent_peers": commaDelimited(privatePeerStr, tendermint.PersistentPeers),
		"seeds":            tendermint.Seeds,
	}

	privateNodeIDs := lo.Map(lo.Values(privatePeers), func(p Peer, _ int) string { return string(p.NodeID) })
	sort.Strings(privateNodeIDs)
	privateIDStr := commaDelimited(privateNodeIDs...)

	privateIDs := commaDelimited(privateIDStr, tendermint.PrivatePeerIDs)
	if v := privateIDs; v != "" {
		p2p["private_peer_ids"] = v
	}

	unconditionalIDs := commaDelimited(privateIDStr, tendermint.UnconditionalPeerIDs)
	if v := unconditionalIDs; v != "" {
		p2p["unconditional_peer_ids"] = v
	}

	if v := tendermint.MaxInboundPeers; v != nil {
		p2p["max_num_inbound_peers"] = tendermint.MaxInboundPeers
	}
	if v := tendermint.MaxOutboundPeers; v != nil {
		p2p["max_num_outbound_peers"] = tendermint.MaxOutboundPeers
	}
	if v := peers.Get(instance, crd.Namespace).ExternalAddress; v != "" {
		p2p["external_address"] = v
	}

	base["p2p"] = p2p

	if v := tendermint.CorsAllowedOrigins; v != nil {
		base["rpc"] = decodedToml{"cors_allowed_origins": v}
	}

	dst := defaultTendermint()

	mergemap.Merge(dst, base)

	if overrides := tendermint.TomlOverrides; overrides != nil {
		var decoded decodedToml
		_, err := toml.Decode(*overrides, &decoded)
		if err != nil {
			return fmt.Errorf("invalid toml in tendermint overrides: %w", err)
		}
		mergemap.Merge(dst, decoded)
	}

	if err := toml.NewEncoder(buf).Encode(dst); err != nil {
		return err
	}
	cmData[configOverlayFile] = buf.String()
	return nil
}

func commaDelimited(s ...string) string {
	return strings.Join(lo.Compact(s), ",")
}

func addAppToml(buf *bytes.Buffer, cmData map[string]string, app cosmosv1.SDKAppConfig) error {
	base := make(decodedToml)
	base["minimum-gas-prices"] = app.MinGasPrice
	// Note: The name discrepancy "enable" vs. "enabled" is intentional; a known inconsistency within the app.toml.
	base["api"] = decodedToml{"enabled-unsafe-cors": app.APIEnableUnsafeCORS}
	base["grpc-web"] = decodedToml{"enable-unsafe-cors": app.GRPCWebEnableUnsafeCORS}

	if v := app.HaltHeight; v != nil {
		base["halt-height"] = v
	}

	if pruning := app.Pruning; pruning != nil {
		intStr := func(n *uint32) string {
			v := valOrDefault(n, ptr(uint32(0)))
			return strconv.FormatUint(uint64(*v), 10)
		}
		base["pruning"] = pruning.Strategy
		base["pruning-interval"] = intStr(pruning.Interval)
		base["pruning-keep-every"] = intStr(pruning.KeepEvery)
		base["pruning-keep-recent"] = intStr(pruning.KeepRecent)
		base["min-retain-blocks"] = valOrDefault(pruning.MinRetainBlocks, ptr(uint32(0)))
	}

	dst := defaultApp()
	mergemap.Merge(dst, base)

	if overrides := app.TomlOverrides; overrides != nil {
		var decoded decodedToml
		_, err := toml.Decode(*overrides, &decoded)
		if err != nil {
			return fmt.Errorf("invalid toml in app overrides: %w", err)
		}
		mergemap.Merge(dst, decoded)
	}

	if err := toml.NewEncoder(buf).Encode(dst); err != nil {
		return err
	}
	cmData[appOverlayFile] = buf.String()
	return nil
}
