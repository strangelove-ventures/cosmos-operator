package fullnode

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/peterbourgon/mergemap"
	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configOverlayFile = "config-overlay.toml"
	appOverlayFile    = "app-overlay.toml"
)

// BuildConfigMaps creates a ConfigMap with configuration to be mounted as files into containers.
// Currently, the config.toml (for Tendermint) and app.toml (for the Cosmos SDK).
func BuildConfigMaps(crd *cosmosv1.CosmosFullNode, p2p ExternalAddresses) ([]*corev1.ConfigMap, error) {
	var (
		buf = bufPool.Get().(*bytes.Buffer)
		cms = make([]*corev1.ConfigMap, crd.Spec.Replicas)
	)
	defer bufPool.Put(buf)
	defer buf.Reset()

	for i := int32(0); i < crd.Spec.Replicas; i++ {
		data := make(map[string]string)
		instance := instanceName(crd, i)
		if err := addConfigToml(buf, data, crd, p2p[instance]); err != nil {
			return nil, err
		}
		buf.Reset()
		if err := addAppToml(buf, data, crd.Spec.ChainSpec.App); err != nil {
			return nil, err
		}
		buf.Reset()

		cm := corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      instanceName(crd, i),
				Namespace: crd.Namespace,
				Labels: defaultLabels(crd,
					kube.InstanceLabel, instanceName(crd, i),
					kube.RevisionLabel, configMapRevisionHash(crd, p2p),
				),
			},
		}
		cm.Data = data

		kube.NormalizeMetadata(&cm.ObjectMeta)
		cms[i] = &cm
	}

	return cms, nil
}

func configMapRevisionHash(crd *cosmosv1.CosmosFullNode, addresses ExternalAddresses) string {
	h := fnv.New32()
	mustWrite(h, mustMarshalJSON(crd.Spec.ChainSpec))
	mustWrite(h, mustMarshalJSON(crd.Spec.PodTemplate.Image))
	mustWrite(h, mustMarshalJSON(crd.Spec.Type))

	vals := lo.MapToSlice(addresses, func(v, k string) string {
		return v + k
	})
	sort.Strings(vals)
	mustWrite(h, strings.Join(vals, ""))

	return hex.EncodeToString(h.Sum(nil))
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

func addConfigToml(buf *bytes.Buffer, cmData map[string]string, crd *cosmosv1.CosmosFullNode, externalAddress string) error {
	var (
		spec = crd.Spec.ChainSpec
		base = make(decodedToml)
	)

	if crd.Spec.Type == cosmosv1.FullNodeSentry {
		base["priv_validator_laddr"] = fmt.Sprintf("tcp://0.0.0.0:%d", privvalPort)
	}
	if v := spec.LogLevel; v != nil {
		base["log_level"] = v
	}
	if v := spec.LogFormat; v != nil {
		base["log_format"] = v
	}

	tendermint := spec.Tendermint
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
	if v := externalAddress; v != "" {
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
