package fullnode

import (
	"bytes"
	_ "embed"
	"fmt"
	blockchain_toml "github.com/bharvest-devops/blockchain-toml"
	"github.com/pelletier/go-toml"
	"strings"

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

func getEmptyNamadaConfig() blockchain_toml.NamadaConfigFile {
	return blockchain_toml.NamadaConfigFile{
		Ledger: blockchain_toml.NamadaLedger{
			Shell: blockchain_toml.NamadaShell{},
			Cometbft: blockchain_toml.NamadaCometbft{
				RPC:             blockchain_toml.NamadaRPC{},
				P2P:             blockchain_toml.NamadaP2P{},
				Mempool:         blockchain_toml.NamadaMempool{},
				Consensus:       blockchain_toml.NamadaConsensus{},
				Storage:         blockchain_toml.NamadaStorage{},
				TxIndex:         blockchain_toml.NamadaTxIndex{},
				Instrumentation: blockchain_toml.NamadaInstrumentation{},
				Statesync:       blockchain_toml.NamadaStatesync{},
				Fastsync:        blockchain_toml.NamadaFastsync{},
			},
			EthereumBridge: blockchain_toml.NamadaEthereumBridge{},
		},
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
			config := getEmptyNamadaConfig()
			configBytes, err := addNamadaConfigToml(&config, crd, instance, peers)
			// You should remove moniker at configBytes
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
	var (
		cosmosConfigFile blockchain_toml.CosmosConfigFile
	)

	spec := crd.Spec.ChainSpec
	comet := spec.Comet

	if comet != nil {
		cosmosConfigFile = comet.ToCosmosConfig()
	} else {
		cosmosConfigFile = *config
	}

	config = &cosmosConfigFile

	// Prepare private peers for nodes in cluster
	privatePeers := peers.Except(instance, crd.Namespace)
	privatePeerStr := commaDelimited(stringListToStringPointerList(privatePeers.AllPrivate())...)
	privateIDStr := commaDelimited(stringListToStringPointerList(privatePeers.NodeIDs())...)

	var privateIDs, persistentPeers, unconditionalIDs string

	privateIDs = commaDelimited(&privateIDStr, config.P2P.PrivatePeerIds)
	config.P2P.PrivatePeerIds = &privateIDs

	persistentPeers = commaDelimited(&privatePeerStr, config.P2P.PersistentPeers)
	config.P2P.PersistentPeers = &persistentPeers

	unconditionalIDs = commaDelimited(&privateIDStr, config.P2P.UnconditionalPeerIds)
	config.P2P.UnconditionalPeerIds = &unconditionalIDs

	upnpOption := true
	if config.P2P.Upnp == nil {
		// You must set upnpOption true, If you want to connect through k8s service.
		config.P2P.Upnp = &upnpOption
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

	config.Moniker = &instance
	if spec.Comet != nil && spec.Comet.TomlOverrides != nil {
		return config.ExportMergeWithTomlOverrides([]byte(*spec.Comet.TomlOverrides))
	}
	return toml.Marshal(config)
}

func addCosmosAppToml(app *blockchain_toml.CosmosAppFile, crd *cosmosv1.CosmosFullNode) ([]byte, error) {
	var (
		cosmosSDK = crd.Spec.ChainSpec.CosmosSDK
	)

	app.MinimumGasPrices = &cosmosSDK.MinGasPrice
	app.API.EnabledUnsafeCors = &cosmosSDK.APIEnableUnsafeCORS
	app.GrpcWeb.EnableUnsafeCors = &cosmosSDK.GRPCWebEnableUnsafeCORS

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
		return app.ExportMergeWithTomlOverrides([]byte(*cosmosSDK.TomlOverrides))
	}
	return toml.Marshal(app)
}

func addNamadaConfigToml(config *blockchain_toml.NamadaConfigFile, crd *cosmosv1.CosmosFullNode, instance string, peers Peers) ([]byte, error) {
	var (
		namadaCometBFT blockchain_toml.NamadaCometbft
		err            error
	)

	spec := crd.Spec.ChainSpec
	comet := spec.Comet
	if comet != nil {
		namadaCometBFT = comet.ToNamadaComet()
	} else {
		namadaCometBFT = config.Ledger.Cometbft
	}
	config.Ledger.Cometbft = namadaCometBFT

	// Prepare private peers for nodes in cluster
	privatePeers := peers.Except(instance, crd.Namespace)
	privatePeerStr := commaDelimited(stringListToStringPointerList(privatePeers.AllPrivate())...)
	privateIDStr := commaDelimited(stringListToStringPointerList(privatePeers.NodeIDs())...)

	var privateIDs, persistentPeers, unconditionalIDs string

	privateIDs = commaDelimited(&privateIDStr, config.Ledger.Cometbft.P2P.PrivatePeerIds)
	config.Ledger.Cometbft.P2P.PrivatePeerIds = &privateIDs

	persistentPeers = commaDelimited(&privatePeerStr, config.Ledger.Cometbft.P2P.PersistentPeers)
	config.Ledger.Cometbft.P2P.PersistentPeers = &persistentPeers

	unconditionalIDs = commaDelimited(&privateIDStr, config.Ledger.Cometbft.P2P.UnconditionalPeerIds)
	config.Ledger.Cometbft.P2P.UnconditionalPeerIds = &unconditionalIDs

	upnpOption := true
	if config.Ledger.Cometbft.P2P.Upnp == nil {
		// You must set upnpOption true, If you want to connect through k8s service.
		config.Ledger.Cometbft.P2P.Upnp = &upnpOption
	}

	var externalOverride bool
	if crd.Spec.InstanceOverrides != nil {
		if override, ok := crd.Spec.InstanceOverrides[instance]; ok && override.ExternalAddress != nil {
			config.Ledger.Cometbft.P2P.ExternalAddress = override.ExternalAddress
			externalOverride = true
		}
	}

	if !externalOverride {
		if v := peers.Get(instance, crd.Namespace).ExternalAddress; v != "" {
			config.Ledger.Cometbft.P2P.ExternalAddress = &v
		}
	}

	if crd.Spec.Type == cosmosv1.Sentry {
		privVal := fmt.Sprintf("tcp://0.0.0.0:%d", privvalPort)
		config.Ledger.Cometbft.PrivValidatorLaddr = &privVal
		// Disable indexing for sentries; they should not serve queries.
		txIndexer := "null"
		config.Ledger.Cometbft.TxIndex.Indexer = &txIndexer
	}
	if v := spec.LogLevel; v != nil {
		config.Ledger.Cometbft.LogLevel = v
	}
	if v := spec.LogFormat; v != nil {
		config.Ledger.Cometbft.LogFormat = v
	}

	// Prepare for namada specified config
	if crd.Spec.ChainSpec.Namada != nil {
		namadaConfig := crd.Spec.ChainSpec.Namada.ToNamadaConfig()
		err = config.MergeWithConfig(&namadaConfig)
		if err != nil {
			return nil, err
		}
	}

	config.Ledger.ChainID = crd.Spec.ChainSpec.ChainID
	config.Ledger.Shell.BaseDir = ChainHomeDir(crd) + getCometbftDir(crd)

	config.Ledger.Cometbft.Moniker = &instance
	if spec.Comet != nil && spec.Comet.TomlOverrides != nil {
		return config.ExportMergeWithTomlOverrides([]byte(*spec.Comet.TomlOverrides))
	}
	return toml.Marshal(config)
}
