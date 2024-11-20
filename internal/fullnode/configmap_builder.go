package fullnode

import (
	"bytes"
	_ "embed"
	"fmt"
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
// Currently, the config.toml (for Comet) and app.toml (for the Cosmos SDK).
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
		appCfg := crd.Spec.ChainSpec.App
		if len(crd.Spec.ChainSpec.Versions) > 0 {
			instanceHeight := uint64(0)
			if height, ok := crd.Status.Height[instance]; ok {
				instanceHeight = height
			}
			haltHeight := uint64(0)
			for i, v := range crd.Spec.ChainSpec.Versions {
				if v.SetHaltHeight {
					haltHeight = v.UpgradeHeight
				} else {
					haltHeight = 0
				}
				if instanceHeight < v.UpgradeHeight {
					break
				}
				if i == len(crd.Spec.ChainSpec.Versions)-1 {
					haltHeight = 0
				}
			}
			appCfg.HaltHeight = ptr(haltHeight)
		}
		if err := addAppToml(buf, data, appCfg); err != nil {
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

//go:embed toml/comet_default_config.toml
var defaultCometToml []byte

func defaultComet() decodedToml {
	var data decodedToml
	if err := toml.Unmarshal(defaultCometToml, &data); err != nil {
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

	if crd.Spec.Type == cosmosv1.Sentry {
		privVal := fmt.Sprintf("tcp://0.0.0.0:%d", privvalPort)
		base["priv_validator_laddr"] = privVal
		base["priv-validator-laddr"] = privVal
		// Disable indexing for sentries; they should not serve queries.

		txIndex := map[string]string{"indexer": "null"}
		base["tx_index"] = txIndex
		base["tx-index"] = txIndex
	}
	if v := spec.LogLevel; v != nil {
		base["log_level"] = v
		base["log-level"] = v
	}
	if v := spec.LogFormat; v != nil {
		base["log_format"] = v
		base["log-format"] = v
	}

	privatePeers := peers.Except(instance, crd.Namespace)
	privatePeerStr := commaDelimited(privatePeers.AllPrivate()...)
	comet := spec.Comet
	persistentPeers := commaDelimited(privatePeerStr, comet.PersistentPeers)
	p2p := decodedToml{
		"persistent_peers": persistentPeers,
		"persistent-peers": persistentPeers,
		"seeds":            comet.Seeds,
	}

	privateIDStr := commaDelimited(privatePeers.NodeIDs()...)
	privateIDs := commaDelimited(privateIDStr, comet.PrivatePeerIDs)
	if v := privateIDs; v != "" {
		p2p["private_peer_ids"] = v
		p2p["private-peer-ids"] = v
	}

	unconditionalIDs := commaDelimited(privateIDStr, comet.UnconditionalPeerIDs)
	if v := unconditionalIDs; v != "" {
		p2p["unconditional_peer_ids"] = v
		p2p["unconditional-peer-ids"] = v
	}

	if v := comet.MaxInboundPeers; v != nil {
		p2p["max_num_inbound_peers"] = comet.MaxInboundPeers
		p2p["max-num-inbound-peers"] = comet.MaxInboundPeers
	}
	if v := comet.MaxOutboundPeers; v != nil {
		p2p["max_num_outbound_peers"] = comet.MaxOutboundPeers
		p2p["max-num-outbound-peers"] = comet.MaxOutboundPeers
	}

	var externalOverride bool
	if crd.Spec.InstanceOverrides != nil {
		if override, ok := crd.Spec.InstanceOverrides[instance]; ok && override.ExternalAddress != nil {
			addr := *override.ExternalAddress
			p2p["external_address"] = addr
			p2p["external-address"] = addr
			externalOverride = true
		}
	}

	if !externalOverride {
		if v := peers.Get(instance, crd.Namespace).ExternalAddress; v != "" {
			p2p["external_address"] = v
			p2p["external-address"] = v
		}
	}

	base["p2p"] = p2p

	if v := comet.CorsAllowedOrigins; v != nil {
		base["rpc"] = decodedToml{
			"cors_allowed_origins": v,
			"cors-allowed-origins": v,
		}
	}

	dst := defaultComet()

	mergemap.Merge(dst, base)

	if overrides := comet.TomlOverrides; overrides != nil {
		var decoded decodedToml
		_, err := toml.Decode(*overrides, &decoded)
		if err != nil {
			return fmt.Errorf("invalid toml in comet overrides: %w", err)
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
