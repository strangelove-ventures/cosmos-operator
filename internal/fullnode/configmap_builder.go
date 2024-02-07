package fullnode

import (
	"bytes"
	_ "embed"
	"fmt"
	blockchain_toml "github.com/bharvest-devops/blockchain-toml"
	"strings"

	"github.com/BurntSushi/toml"
	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	"github.com/bharvest-devops/cosmos-operator/internal/diff"
	"github.com/bharvest-devops/cosmos-operator/internal/kube"
	"github.com/samber/lo"
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

		if crd.Spec.ChainSpec.ChainType == chainTypeNamada {
			config := blockchain_toml.NamadaConfigFile{}
			configBytes, err := addNamadaConfigToml(&config, crd, instance, peers)
			if err != nil {
				return nil, err
			}
			data[configOverlayFile] = string(configBytes)

		} else {
			config := blockchain_toml.CosmosConfigFile{}
			configBytes, err := addCosmosConfigToml(&config, crd, instance, peers)
			if err != nil {
				return nil, err
			}
			data[configOverlayFile] = string(configBytes)

			app := blockchain_toml.CosmosAppFile{}
			appTomlBytes, err := addCosmosAppToml(&app, crd)
			if err != nil {
				return nil, err
			}
			data[appOverlayFile] = string(appTomlBytes)
		}

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

func commaDelimited(s ...string) string {
	return strings.Join(lo.Compact(s), ",")
}

func addCosmosConfigToml(config *blockchain_toml.CosmosConfigFile, crd *cosmosv1.CosmosFullNode, instance string, peers Peers) ([]byte, error) {
	spec := crd.Spec.ChainSpec
	comet := spec.Comet

	privatePeers := peers.Except(instance, crd.Namespace)
	privatePeerStr := commaDelimited(privatePeers.AllPrivate()...)
	privateIDStr := commaDelimited(privatePeers.NodeIDs()...)
	privateIDs := commaDelimited(privateIDStr, *comet.P2P.PrivatePeerIds)

	persistentPeers := commaDelimited(privatePeerStr, *comet.P2P.PersistentPeers)

	unconditionalIDs := commaDelimited(privateIDStr, *comet.P2P.UnconditionalPeerIDs)

	cosmosP2P := blockchain_toml.CosmosP2P{
		Seeds:                comet.P2P.Seeds,
		PersistentPeers:      &persistentPeers,
		PrivatePeerIds:       &privateIDs,
		UnconditionalPeerIds: &unconditionalIDs,
		MaxNumInboundPeers:   comet.P2P.MaxNumInboundPeers,
		MaxNumOutboundPeers:  comet.P2P.MaxNumOutboundPeers,
	}

	var externalOverride bool
	if crd.Spec.InstanceOverrides != nil {
		if override, ok := crd.Spec.InstanceOverrides[instance]; ok && override.ExternalAddress != nil {
			addr := *override.ExternalAddress
			*cosmosP2P.ExternalAddress = addr
			externalOverride = true
		}
	}

	if !externalOverride {
		if v := peers.Get(instance, crd.Namespace).ExternalAddress; v != "" {
			*cosmosP2P.ExternalAddress = v
		}
	}

	cosmosRPC := blockchain_toml.CosmosRPC{
		CorsAllowedOrigins: comet.RPC.CorsAllowedOrigins,
	}

	*config = blockchain_toml.CosmosConfigFile{
		Moniker: &instance,
		P2P:     &cosmosP2P,
		RPC:     &cosmosRPC,
	}

	var err error
	overrideConfig := blockchain_toml.CosmosConfigFile{}

	P2PTomlOverrides := blockchain_toml.CosmosConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.P2P.TomlOverrides), &P2PTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&P2PTomlOverrides)
	if err != nil {
		return nil, err
	}

	RPCTomlOverrides := blockchain_toml.CosmosConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.P2P.TomlOverrides), &RPCTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&RPCTomlOverrides)
	if err != nil {
		return nil, err
	}

	MempoolTomlOverrides := blockchain_toml.CosmosConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.Mempool.TomlOverrides), &MempoolTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&MempoolTomlOverrides)
	if err != nil {
		return nil, err
	}

	ConsensusTomlOverrides := blockchain_toml.CosmosConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.Consensus.TomlOverrides), &ConsensusTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&ConsensusTomlOverrides)
	if err != nil {
		return nil, err
	}

	TxIndexTomlOverrides := blockchain_toml.CosmosConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.TxIndex.TomlOverrides), &TxIndexTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&TxIndexTomlOverrides)
	if err != nil {
		return nil, err
	}

	InstrumentationTomlOverrides := blockchain_toml.CosmosConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.Instrumentation.TomlOverrides), &InstrumentationTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&InstrumentationTomlOverrides)
	if err != nil {
		return nil, err
	}

	StatesyncTomlOverrides := blockchain_toml.CosmosConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.Statesync.TomlOverrides), &StatesyncTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&StatesyncTomlOverrides)
	if err != nil {
		return nil, err
	}

	if crd.Spec.Type == cosmosv1.Sentry {
		privVal := fmt.Sprintf("tcp://0.0.0.0:%d", privvalPort)
		*config.PrivValidatorLaddr = privVal
		// Disable indexing for sentries; they should not serve queries.

		*overrideConfig.TxIndex.Indexer = "null"
	}
	if v := spec.LogLevel; v != nil {
		overrideConfig.LogLevel = v
	}
	if v := spec.LogFormat; v != nil {
		overrideConfig.LogFormat = v
	}

	if err = config.MergeWithConfig(&overrideConfig); err != nil {
		return nil, err
	}

	return config.ExportMergeWithDefault()
}

func addCosmosAppToml(app *blockchain_toml.CosmosAppFile, crd *cosmosv1.CosmosFullNode) ([]byte, error) {
	spec := crd.Spec.ChainSpec

	api := blockchain_toml.API{
		EnabledUnsafeCors: &spec.CosmosSDK.APIEnableUnsafeCORS,
	}
	grpcWeb := blockchain_toml.GrpcWeb{
		EnableUnsafeCors: &spec.CosmosSDK.GRPCWebEnableUnsafeCORS,
	}
	pruningKeepRecent := fmt.Sprint(spec.CosmosSDK.Pruning.KeepRecent)
	pruningKeepEvery := fmt.Sprint(spec.CosmosSDK.Pruning.KeepEvery)

	*app = blockchain_toml.CosmosAppFile{
		MinimumGasPrices:  &spec.CosmosSDK.MinGasPrice,
		API:               &api,
		GrpcWeb:           &grpcWeb,
		Pruning:           (*string)(&spec.CosmosSDK.Pruning.Strategy),
		PruningKeepRecent: &pruningKeepRecent,
		PruningKeepEvery:  &pruningKeepEvery,
		HaltHeight:        spec.CosmosSDK.HaltHeight,
	}

	tomlOverrides := spec.CosmosSDK.TomlOverrides
	overrideConfig := blockchain_toml.CosmosAppFile{}

	if err := toml.Unmarshal([]byte(*tomlOverrides), overrideConfig); err != nil {
		return nil, err
	}

	if err := app.MergeWithConfig(&overrideConfig); err != nil {
		return nil, err
	}

	return app.ExportMergeWithDefault()
}

func addNamadaConfigToml(config *blockchain_toml.NamadaConfigFile, crd *cosmosv1.CosmosFullNode, instance string, peers Peers) ([]byte, error) {
	spec := crd.Spec.ChainSpec
	comet := spec.Comet

	privatePeers := peers.Except(instance, crd.Namespace)
	privatePeerStr := commaDelimited(privatePeers.AllPrivate()...)
	privateIDStr := commaDelimited(privatePeers.NodeIDs()...)
	privateIDs := commaDelimited(privateIDStr, *comet.P2P.PrivatePeerIds)

	persistentPeers := commaDelimited(privatePeerStr, *comet.P2P.PersistentPeers)

	unconditionalIDs := commaDelimited(privateIDStr, *comet.P2P.UnconditionalPeerIDs)

	namadaP2P := blockchain_toml.NamadaP2P{
		Seeds:                comet.P2P.Seeds,
		PersistentPeers:      &persistentPeers,
		PrivatePeerIds:       &privateIDs,
		UnconditionalPeerIds: &unconditionalIDs,
		MaxNumInboundPeers:   comet.P2P.MaxNumInboundPeers,
		MaxNumOutboundPeers:  comet.P2P.MaxNumOutboundPeers,
	}

	var externalOverride bool
	if crd.Spec.InstanceOverrides != nil {
		if override, ok := crd.Spec.InstanceOverrides[instance]; ok && override.ExternalAddress != nil {
			addr := *override.ExternalAddress
			*namadaP2P.ExternalAddress = addr
			externalOverride = true
		}
	}

	if !externalOverride {
		if v := peers.Get(instance, crd.Namespace).ExternalAddress; v != "" {
			*namadaP2P.ExternalAddress = v
		}
	}

	namadaRPC := blockchain_toml.NamadaRPC{
		CorsAllowedOrigins: comet.RPC.CorsAllowedOrigins,
	}

	cometBFT := blockchain_toml.NamadaCometbft{
		Moniker: &instance,
		P2P:     &namadaP2P,
		RPC:     &namadaRPC,
	}

	namadaLedger := blockchain_toml.NamadaLedger{
		Cometbft: &cometBFT,
	}

	*config = blockchain_toml.NamadaConfigFile{
		Ledger: &namadaLedger,
	}

	var err error
	overrideConfig := blockchain_toml.NamadaConfigFile{}

	P2PTomlOverrides := blockchain_toml.NamadaConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.P2P.TomlOverrides), &P2PTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&P2PTomlOverrides)
	if err != nil {
		return nil, err
	}

	RPCTomlOverrides := blockchain_toml.NamadaConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.P2P.TomlOverrides), &RPCTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&RPCTomlOverrides)
	if err != nil {
		return nil, err
	}

	MempoolTomlOverrides := blockchain_toml.NamadaConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.Mempool.TomlOverrides), &MempoolTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&MempoolTomlOverrides)
	if err != nil {
		return nil, err
	}

	ConsensusTomlOverrides := blockchain_toml.NamadaConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.Consensus.TomlOverrides), &ConsensusTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&ConsensusTomlOverrides)
	if err != nil {
		return nil, err
	}

	TxIndexTomlOverrides := blockchain_toml.NamadaConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.TxIndex.TomlOverrides), &TxIndexTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&TxIndexTomlOverrides)
	if err != nil {
		return nil, err
	}

	InstrumentationTomlOverrides := blockchain_toml.NamadaConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.Instrumentation.TomlOverrides), &InstrumentationTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&InstrumentationTomlOverrides)
	if err != nil {
		return nil, err
	}

	StatesyncTomlOverrides := blockchain_toml.NamadaConfigFile{}
	if err = toml.Unmarshal([]byte(*comet.Statesync.TomlOverrides), &StatesyncTomlOverrides); err != nil {
		return nil, err
	}
	err = overrideConfig.MergeWithConfig(&StatesyncTomlOverrides)
	if err != nil {
		return nil, err
	}

	if crd.Spec.Type == cosmosv1.Sentry {
		privVal := fmt.Sprintf("tcp://0.0.0.0:%d", privvalPort)
		*config.Ledger.Cometbft.PrivValidatorLaddr = privVal
		// Disable indexing for sentries; they should not serve queries.

		*overrideConfig.Ledger.Cometbft.TxIndex.Indexer = "null"
	}
	if v := spec.LogLevel; v != nil {
		overrideConfig.Ledger.Cometbft.LogLevel = v
	}
	if v := spec.LogFormat; v != nil {
		overrideConfig.Ledger.Cometbft.LogFormat = v
	}

	if err = config.MergeWithConfig(&overrideConfig); err != nil {
		return nil, err
	}

	return config.ExportMergeWithDefault()
}
