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

func getEmptyCosmosConfig() blockchain_toml.CosmosConfigFile {
	configRPC := blockchain_toml.CosmosRPC{}
	configP2P := blockchain_toml.CosmosP2P{}
	configMempool := blockchain_toml.CosmosMempool{}
	configStatesync := blockchain_toml.CosmosStatesync{}
	configBlocksync := blockchain_toml.CosmosBlocksync{}
	configConsensus := blockchain_toml.CosmosConsensus{}
	configStorage := blockchain_toml.CosmosStorage{}
	configTxIndex := blockchain_toml.CosmosTxIndex{}
	configInstrumentation := blockchain_toml.CosmosInstrumentation{}

	return blockchain_toml.CosmosConfigFile{
		RPC:             configRPC,
		P2P:             configP2P,
		Mempool:         configMempool,
		Statesync:       configStatesync,
		Blocksync:       configBlocksync,
		Consensus:       configConsensus,
		Storage:         configStorage,
		TxIndex:         configTxIndex,
		Instrumentation: configInstrumentation,
	}
}

func getEmptyCosmosApp() blockchain_toml.CosmosAppFile {
	appTelementry := blockchain_toml.Telemetry{}
	appAPI := blockchain_toml.API{}
	appRosetta := blockchain_toml.Rosetta{}
	appGrpc := blockchain_toml.Grpc{}
	appGrpcWeb := blockchain_toml.GrpcWeb{}
	appStatesync := blockchain_toml.StateSync{}

	return blockchain_toml.CosmosAppFile{
		Telemetry: appTelementry,
		API:       appAPI,
		Rosetta:   appRosetta,
		Grpc:      appGrpc,
		GrpcWeb:   appGrpcWeb,
		StateSync: appStatesync,
	}
}

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
			if crd.Spec.ChainSpec.Comet != nil {

			}
			config := blockchain_toml.NamadaConfigFile{}
			configBytes, err := addNamadaConfigToml(&config, crd, instance, peers)
			if err != nil {
				return nil, err
			}
			data[configOverlayFile] = string(configBytes)

		} else {

			config := getEmptyCosmosConfig()
			configBytes, err := addCosmosConfigToml(&config, crd, instance, peers)
			if err != nil {
				return nil, err
			}
			data[configOverlayFile] = string(configBytes)

			app := getEmptyCosmosApp()
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

func commaDelimited(str ...*string) string {
	var compactList []string
	for _, s := range str {
		if s != nil {
			compactList = append(compactList, *s)
		}
	}
	return strings.Join(lo.Compact(compactList), ",")
}

func stringListToStringPointerList(str []string) []*string {
	var strPtrList []*string
	for _, p := range str {
		t := p
		strPtrList = append(strPtrList, &t)
	}
	return strPtrList
}

func addCosmosConfigToml(config *blockchain_toml.CosmosConfigFile, crd *cosmosv1.CosmosFullNode, instance string, peers Peers) ([]byte, error) {
	var err error

	spec := crd.Spec.ChainSpec
	comet := spec.Comet

	//config.Moniker = &instance

	if comet.RPC != nil {
		cosmosRPC := comet.RPC.ToCosmosRPC()

		if comet.RPC.TomlOverrides != nil {
			RPCTomlOverrides := blockchain_toml.CosmosConfigFile{}
			if err = toml.Unmarshal([]byte(*comet.P2P.TomlOverrides), &RPCTomlOverrides); err != nil {
				return nil, err
			}
			err = config.MergeWithConfig(RPCTomlOverrides)
			if err != nil {
				return nil, err
			}
		}

		config.RPC = cosmosRPC
	}

	if comet.P2P != nil {
		cosmosP2P := comet.P2P.ToCosmosP2P()

		if comet.P2P.TomlOverrides != nil {
			P2PTomlOverrides := blockchain_toml.CosmosConfigFile{}
			if err = toml.Unmarshal([]byte(*comet.P2P.TomlOverrides), &P2PTomlOverrides); err != nil {
				return nil, err
			}
			err = config.MergeWithConfig(P2PTomlOverrides)
			if err != nil {
				return nil, err
			}
		}

		// Prepare private peers for nodes in cluster
		privatePeers := peers.Except(instance, crd.Namespace)
		privatePeerStr := commaDelimited(stringListToStringPointerList(privatePeers.AllPrivate())...)
		privateIDStr := commaDelimited(stringListToStringPointerList(privatePeers.NodeIDs())...)

		var privateIDs, persistentPeers, unconditionalIDs string

		if cosmosP2P.PrivatePeerIds != nil {
			privateIDs = commaDelimited(&privateIDStr, cosmosP2P.PrivatePeerIds)
			cosmosP2P.PrivatePeerIds = &privateIDs
		}
		if cosmosP2P.PersistentPeers != nil {
			persistentPeers = commaDelimited(&privatePeerStr, cosmosP2P.PersistentPeers)
			cosmosP2P.PersistentPeers = &persistentPeers
		}
		if cosmosP2P.UnconditionalPeerIds != nil {
			unconditionalIDs = commaDelimited(&privateIDStr, cosmosP2P.UnconditionalPeerIds)
			cosmosP2P.UnconditionalPeerIds = &unconditionalIDs
		}

		config.P2P = cosmosP2P
	}

	var externalOverride bool
	if crd.Spec.InstanceOverrides != nil {
		if override, ok := crd.Spec.InstanceOverrides[instance]; ok && override.ExternalAddress != nil {
			config.P2P.ExternalAddress = override.ExternalAddress
			externalOverride = true
		}
	}

	if !externalOverride {
		if v := peers.Get(instance, crd.Namespace).ExternalAddress; v != "" {
			config.P2P.ExternalAddress = &v
		}
	}

	if comet.Mempool != nil {
		if comet.Mempool.TomlOverrides != nil {
			mempoolTomlOverrides := blockchain_toml.CosmosConfigFile{}
			if err = toml.Unmarshal([]byte(*comet.Mempool.TomlOverrides), &mempoolTomlOverrides); err != nil {
				return nil, err
			}
			err = config.MergeWithConfig(mempoolTomlOverrides)
			if err != nil {
				return nil, err
			}
		}
	}

	if comet.Consensus != nil {
		cosmosConsensus := comet.Consensus.ToCosmosConsensus()

		if comet.Consensus.TomlOverrides != nil {
			consensusTomlOverrides := blockchain_toml.CosmosConfigFile{}
			if err = toml.Unmarshal([]byte(*comet.Consensus.TomlOverrides), &consensusTomlOverrides); err != nil {
				return nil, err
			}
			err = config.MergeWithConfig(consensusTomlOverrides)
			if err != nil {
				return nil, err
			}
		}
		config.Consensus = cosmosConsensus
	}

	if comet.Storage != nil {
		cosmosStorage := comet.Storage.ToCosmosStorage()
		config.Storage = cosmosStorage
	}

	if comet.TxIndex != nil {
		cosmosTxIndex := comet.TxIndex.ToCosmosTxIndex()

		if comet.TxIndex.TomlOverrides != nil {
			txIndexTomlOverrides := blockchain_toml.CosmosConfigFile{}
			if err = toml.Unmarshal([]byte(*comet.TxIndex.TomlOverrides), &txIndexTomlOverrides); err != nil {
				return nil, err
			}
			err = config.MergeWithConfig(txIndexTomlOverrides)
			if err != nil {
				return nil, err
			}
		}
		config.TxIndex = cosmosTxIndex
	}

	if comet.Instrumentation != nil {
		cosmosInstrumentation := comet.Instrumentation.ToCosmosInstrumentation()

		if comet.Instrumentation.TomlOverrides != nil {
			instrumentationTomlOverrides := blockchain_toml.CosmosConfigFile{}
			if err = toml.Unmarshal([]byte(*comet.Instrumentation.TomlOverrides), &instrumentationTomlOverrides); err != nil {
				return nil, err
			}
			err = config.MergeWithConfig(instrumentationTomlOverrides)
			if err != nil {
				return nil, err
			}
		}
		config.Instrumentation = cosmosInstrumentation
	}

	if comet.Statesync != nil {
		cosmosStatesync := comet.Statesync.ToCosmosStatesync()

		if comet.Statesync.TomlOverrides != nil {
			statesyncTomlOverrides := blockchain_toml.CosmosConfigFile{}
			if err = toml.Unmarshal([]byte(*comet.Statesync.TomlOverrides), &statesyncTomlOverrides); err != nil {
				return nil, err
			}
			err = config.MergeWithConfig(statesyncTomlOverrides)
			if err != nil {
				return nil, err
			}
		}
		config.Statesync = cosmosStatesync
	}

	if crd.Spec.Type == cosmosv1.Sentry {
		privVal := fmt.Sprintf("tcp://0.0.0.0:%d", privvalPort)
		config.PrivValidatorLaddr = &privVal
		// Disable indexing for sentries; they should not serve queries.
		txIndexer := "null"
		config.TxIndex.Indexer = &txIndexer
	}
	if v := spec.LogLevel; v != nil {
		config.LogLevel = v
	}
	if v := spec.LogFormat; v != nil {
		config.LogFormat = v
	}

	if spec.Comet.TomlOverrides != nil {
		if err = config.MergeWithDefault(); err != nil {
			return nil, err
		}
		return config.ExportMergeWithTomlOverrides([]byte(*spec.Comet.TomlOverrides))
	}
	return config.ExportMergeWithDefault()
}

func addCosmosAppToml(app *blockchain_toml.CosmosAppFile, crd *cosmosv1.CosmosFullNode) ([]byte, error) {
	var (
		err       error
		cosmosSDK = crd.Spec.ChainSpec.CosmosSDK
	)

	app.MinimumGasPrices = &cosmosSDK.MinGasPrice

	if &cosmosSDK.APIEnableUnsafeCORS != nil {
		app.API.EnabledUnsafeCors = &cosmosSDK.APIEnableUnsafeCORS
	}

	if &cosmosSDK.GRPCWebEnableUnsafeCORS != nil {
		app.API.EnabledUnsafeCors = &cosmosSDK.GRPCWebEnableUnsafeCORS
	}

	var pruningStrategy, pruningInterval, pruningKeepRecent, pruningKeepEvery string
	var pruningMinRetainBlocks int

	if cosmosSDK.Pruning != nil {
		if cosmosSDK.Pruning.Strategy != "" {
			pruningStrategy = (string)(cosmosSDK.Pruning.Strategy)
			app.Pruning = &pruningStrategy
		}

		if cosmosSDK.Pruning.Interval != nil {
			pruningInterval = fmt.Sprint(*cosmosSDK.Pruning.Interval)
			app.PruningInterval = &pruningInterval
		}

		if cosmosSDK.Pruning.KeepRecent != nil {
			pruningKeepRecent = fmt.Sprint(*cosmosSDK.Pruning.KeepRecent)
			app.PruningKeepRecent = &pruningKeepRecent
		}

		if cosmosSDK.Pruning.KeepEvery != nil {
			pruningKeepEvery = fmt.Sprint(*cosmosSDK.Pruning.KeepEvery)
			app.PruningKeepEvery = &pruningKeepEvery
		}

		if cosmosSDK.Pruning.MinRetainBlocks != nil {
			pruningMinRetainBlocks = int(*cosmosSDK.Pruning.MinRetainBlocks)
			app.MinRetainBlocks = &pruningMinRetainBlocks
		}
	}

	if cosmosSDK.HaltHeight != nil {
		app.HaltHeight = cosmosSDK.HaltHeight
	}

	if cosmosSDK.TomlOverrides != nil {
		if err = app.MergeWithDefault(); err != nil {
			return nil, err
		}
		return app.ExportMergeWithTomlOverrides([]byte(*cosmosSDK.TomlOverrides))
	}
	return app.ExportMergeWithDefault()
}

func addNamadaConfigToml(config *blockchain_toml.NamadaConfigFile, crd *cosmosv1.CosmosFullNode, instance string, peers Peers) ([]byte, error) {
	var err error

	spec := crd.Spec.ChainSpec
	comet := spec.Comet

	//config.Ledger
	//if comet.RPC != nil {
	//	RPC := comet.RPC.to
	//
	//}

	//comet.RPC
	namadaRPC := blockchain_toml.NamadaRPC{}
	privatePeers := peers.Except(instance, crd.Namespace)
	privatePeerStr := commaDelimited(stringListToStringPointerList(privatePeers.AllPrivate())...)
	privateIDStr := commaDelimited(stringListToStringPointerList(privatePeers.NodeIDs())...)
	privateIDs := commaDelimited(&privateIDStr, comet.P2P.PrivatePeerIds)

	persistentPeers := commaDelimited(&privatePeerStr, comet.P2P.PersistentPeers)

	unconditionalIDs := commaDelimited(&privateIDStr, comet.P2P.UnconditionalPeerIDs)

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
			namadaP2P.ExternalAddress = &addr
			externalOverride = true
		}
	}

	if !externalOverride {
		if v := peers.Get(instance, crd.Namespace).ExternalAddress; v != "" {
			namadaP2P.ExternalAddress = &v
		}
	}

	//namadaRPC := blockchain_toml.NamadaRPC{
	//	CorsAllowedOrigins: comet.RPC.CorsAllowedOrigins,
	//}

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

	if spec.Comet.TomlOverrides != nil {
		if err = config.MergeWithDefault(); err != nil {
			return nil, err
		}
		return config.ExportMergeWithTomlOverrides([]byte(*spec.Comet.TomlOverrides))
	}
	return config.ExportMergeWithDefault()
}
