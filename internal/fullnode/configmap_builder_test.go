package fullnode

import (
	"bytes"
	"crypto/ed25519"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	//go:embed testdata/comet.toml
	wantComet string
	//go:embed testdata/comet_defaults.toml
	wantCometDefaults string
	//go:embed testdata/comet_overrides.toml
	wantCometOverrides string

	//go:embed testdata/app.toml
	wantApp string
	//go:embed testdata/app_defaults.toml
	wantAppDefaults string
	//go:embed testdata/app_overrides.toml
	wantAppOverrides string
)

var configMapRequiredEqualKeys = []string{
	configOverlayFile,
	appOverlayFile,
}

func TestBuildConfigMaps(t *testing.T) {
	t.Parallel()

	const namespace = "default"

	t.Run("happy path", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Name = "agoric"
		crd.Namespace = "test"
		crd.Spec.PodTemplate.Image = "agoric:v6.0.0"
		crd.Spec.ChainSpec.Network = "testnet"
		//Default starting ordinal is 0

		nodeKeys, err := getMockNodeKeysForCRD(crd, "")
		require.NoError(t, err)

		cms, err := BuildConfigMaps(&crd, nil, nodeKeys)
		require.NoError(t, err)
		require.Equal(t, crd.Spec.Replicas, int32(len(cms)))

		require.Equal(t, crd.Spec.Ordinals.Start, int32(cms[0].Ordinal())+crd.Spec.Ordinals.Start)
		require.NotEmpty(t, cms[0].Revision())

		cm := cms[0].Object()
		require.Equal(t, fmt.Sprintf("agoric-%d", crd.Spec.Ordinals.Start), cm.Name)
		require.Equal(t, "test", cm.Namespace)
		require.Nil(t, cm.Immutable)

		wantLabels := map[string]string{
			"app.kubernetes.io/created-by": "cosmos-operator",
			"app.kubernetes.io/component":  "CosmosFullNode",
			"app.kubernetes.io/name":       "agoric",
			"app.kubernetes.io/instance":   fmt.Sprintf("%s-%d", crd.Name, crd.Spec.Ordinals.Start),
			"app.kubernetes.io/version":    "v6.0.0",
			"cosmos.strange.love/network":  "testnet",
			"cosmos.strange.love/type":     "FullNode",
		}
		require.Empty(t, cm.Annotations)

		require.Equal(t, wantLabels, cm.Labels)

		cm = cms[1].Object()
		require.Equal(t, fmt.Sprintf("%s-%d", crd.Name, crd.Spec.Ordinals.Start+1), cm.Name)

		require.NotEmpty(t, cms[0].Object().Data)

		cms0Data := cms[0].Object().Data
		cms1Data := cms[1].Object().Data

		for _, key := range configMapRequiredEqualKeys {
			require.Equal(t, cms0Data[key], cms1Data[key])
		}

		nodeKeysFromConfigMap := NodeKeys{}
		for _, cmDiff := range cms {
			currCm := cmDiff.Object()

			nodeKey, nErr := getNodeKeyFromConfigMap(currCm)

			require.NoError(t, nErr)

			nodeKeysFromConfigMap[client.ObjectKey{Name: currCm.Name, Namespace: currCm.Namespace}] = *nodeKey
		}

		crd.Spec.Type = cosmosv1.FullNode
		cms2, err := BuildConfigMaps(&crd, nil, nodeKeysFromConfigMap)

		require.NoError(t, err)
		require.Equal(t, cms, cms2)
	})

	t.Run("happy path with non 0 starting ordinal", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Name = "agoric"
		crd.Namespace = "test"
		crd.Spec.PodTemplate.Image = "agoric:v6.0.0"
		crd.Spec.ChainSpec.Network = "testnet"
		crd.Spec.Ordinals.Start = 2

		nodeKeys, err := getMockNodeKeysForCRD(crd, "")
		require.NoError(t, err)

		cms, err := BuildConfigMaps(&crd, nil, nodeKeys)
		require.NoError(t, err)
		require.Equal(t, crd.Spec.Replicas, int32(len(cms)))

		require.Equal(t, crd.Spec.Ordinals.Start, int32(cms[0].Ordinal())+crd.Spec.Ordinals.Start)
		require.NotEmpty(t, cms[0].Revision())

		cm := cms[0].Object()
		require.Equal(t, fmt.Sprintf("agoric-%d", crd.Spec.Ordinals.Start), cm.Name)
		require.Equal(t, "test", cm.Namespace)
		require.Nil(t, cm.Immutable)

		wantLabels := map[string]string{
			"app.kubernetes.io/created-by": "cosmos-operator",
			"app.kubernetes.io/component":  "CosmosFullNode",
			"app.kubernetes.io/name":       "agoric",
			"app.kubernetes.io/instance":   fmt.Sprintf("%s-%d", crd.Name, crd.Spec.Ordinals.Start),
			"app.kubernetes.io/version":    "v6.0.0",
			"cosmos.strange.love/network":  "testnet",
			"cosmos.strange.love/type":     "FullNode",
		}
		require.Empty(t, cm.Annotations)

		require.Equal(t, wantLabels, cm.Labels)

		cm = cms[1].Object()
		require.Equal(t, fmt.Sprintf("%s-%d", crd.Name, crd.Spec.Ordinals.Start+1), cm.Name)

		require.NotEmpty(t, cms[0].Object().Data)

		cm0Data := cms[0].Object().Data
		cm1Data := cms[1].Object().Data

		for _, key := range configMapRequiredEqualKeys {
			require.Equal(t, cm0Data[key], cm1Data[key])
		}

		nodeKeysFromConfigMap := NodeKeys{}
		for _, cmDiff := range cms {
			currCm := cmDiff.Object()

			nodeKey, nErr := getNodeKeyFromConfigMap(currCm)

			require.NoError(t, nErr)

			nodeKeysFromConfigMap[client.ObjectKey{Name: currCm.Name, Namespace: currCm.Namespace}] = *nodeKey
		}

		crd.Spec.Type = cosmosv1.FullNode
		cms2, err := BuildConfigMaps(&crd, nil, nodeKeysFromConfigMap)

		require.NoError(t, err)
		require.Equal(t, cms, cms2)
	})

	t.Run("long name", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Name = strings.Repeat("chain", 300)
		crd.Spec.ChainSpec.Network = strings.Repeat("network", 300)

		nodeKeys, err := getMockNodeKeysForCRD(crd, "")
		require.NoError(t, err)

		cms, err := BuildConfigMaps(&crd, nil, nodeKeys)
		require.NoError(t, err)
		require.NotEmpty(t, cms)

		for _, cm := range cms {
			test.RequireValidMetadata(t, cm.Object())
		}
	})

	t.Run("config-overlay.toml", func(t *testing.T) {
		crd := defaultCRD()
		crd.Namespace = namespace
		crd.Name = "osmosis"
		crd.Spec.ChainSpec.Network = "mainnet"
		crd.Spec.Replicas = 1
		crd.Spec.ChainSpec.Comet = cosmosv1.CometConfig{
			PersistentPeers: "peer1@1.2.2.2:789,peer2@2.2.2.2:789,peer3@3.2.2.2:789",
			Seeds:           "seed1@1.1.1.1:456,seed2@1.1.1.1:456",
		}

		t.Run("happy path", func(t *testing.T) {
			custom := crd.DeepCopy()
			custom.Spec.Replicas = 1
			custom.Spec.ChainSpec.LogLevel = ptr("debug")
			custom.Spec.ChainSpec.LogFormat = ptr("json")
			custom.Spec.ChainSpec.Comet.CorsAllowedOrigins = []string{"*"}
			custom.Spec.ChainSpec.Comet.MaxInboundPeers = ptr(int32(5))
			custom.Spec.ChainSpec.Comet.MaxOutboundPeers = ptr(int32(15))

			nodeKeys, err := getMockNodeKeysForCRD(*custom, "")
			require.NoError(t, err)

			peers := Peers{
				client.ObjectKey{Namespace: namespace, Name: "osmosis-0"}: {NodeID: "should not see me", PrivateAddress: "should not see me"},
			}
			cms, err := BuildConfigMaps(custom, peers, nodeKeys)
			require.NoError(t, err)

			cm := cms[0].Object()

			require.NotEmpty(t, cm.Data)
			require.Empty(t, cm.BinaryData)

			var (
				got  map[string]any
				want map[string]any
			)
			_, err = toml.Decode(wantComet, &want)
			require.NoError(t, err)

			_, err = toml.Decode(cm.Data["config-overlay.toml"], &got)
			require.NoError(t, err)

			require.Equal(t, want, got)
		})

		t.Run("defaults", func(t *testing.T) {
			nodeKeys, err := getMockNodeKeysForCRD(crd, "")
			require.NoError(t, err)

			cms, err := BuildConfigMaps(&crd, nil, nodeKeys)
			require.NoError(t, err)

			cm := cms[0].Object()

			var (
				got  map[string]any
				want map[string]any
			)
			_, err = toml.Decode(wantCometDefaults, &want)
			require.NoError(t, err)

			_, err = toml.Decode(cm.Data["config-overlay.toml"], &got)
			require.NoError(t, err)

			require.Equal(t, want, got)
		})

		t.Run("with peers", func(t *testing.T) {
			peerCRD := crd.DeepCopy()
			peerCRD.Spec.Replicas = 3
			peerCRD.Spec.ChainSpec.Comet.UnconditionalPeerIDs = "unconditional1,unconditional2"
			peerCRD.Spec.ChainSpec.Comet.PrivatePeerIDs = "private1,private2"
			peers := Peers{
				client.ObjectKey{Namespace: namespace, Name: "osmosis-0"}: {NodeID: "0", PrivateAddress: "0.local:26656"},
				client.ObjectKey{Namespace: namespace, Name: "osmosis-1"}: {NodeID: "1", PrivateAddress: "1.local:26656"},
				client.ObjectKey{Namespace: namespace, Name: "osmosis-2"}: {NodeID: "2", PrivateAddress: "2.local:26656"},
			}

			nodeKeys, err := getMockNodeKeysForCRD(*peerCRD, "")
			require.NoError(t, err)

			cms, err := BuildConfigMaps(peerCRD, peers, nodeKeys)
			require.NoError(t, err)
			require.Len(t, cms, 3)

			for i, tt := range []struct {
				WantPersistent string
				WantIDs        string
			}{
				{"1@1.local:26656,2@2.local:26656", "1,2"},
				{"0@0.local:26656,2@2.local:26656", "0,2"},
				{"0@0.local:26656,1@1.local:26656", "0,1"},
			} {
				cm := cms[i].Object()
				var got map[string]any
				_, err = toml.Decode(cm.Data["config-overlay.toml"], &got)
				require.NoError(t, err, i)

				p2p := got["p2p"].(map[string]any)

				require.Equal(t, tt.WantPersistent+",peer1@1.2.2.2:789,peer2@2.2.2.2:789,peer3@3.2.2.2:789", p2p["persistent_peers"], i)
				require.Equal(t, tt.WantIDs+",private1,private2", p2p["private_peer_ids"], i)
				require.Equal(t, tt.WantIDs+",unconditional1,unconditional2", p2p["unconditional_peer_ids"], i)
			}
		})

		t.Run("validator sentry", func(t *testing.T) {
			sentry := crd.DeepCopy()
			sentry.Spec.Type = cosmosv1.Sentry

			nodeKeys, err := getMockNodeKeysForCRD(*sentry, "")
			require.NoError(t, err)

			cms, err := BuildConfigMaps(sentry, nil, nodeKeys)
			require.NoError(t, err)

			cm := cms[0].Object()

			var got map[string]any
			_, err = toml.Decode(cm.Data["config-overlay.toml"], &got)
			require.NoError(t, err)
			require.NotEmpty(t, got)

			require.Equal(t, "tcp://0.0.0.0:1234", got["priv_validator_laddr"])
			require.Equal(t, "null", got["tx_index"].(map[string]any)["indexer"])
		})

		t.Run("overrides", func(t *testing.T) {
			overrides := crd.DeepCopy()
			overrides.Namespace = namespace
			overrides.Spec.ChainSpec.Comet.CorsAllowedOrigins = []string{"should not see me"}
			overrides.Spec.ChainSpec.Comet.TomlOverrides = ptr(`
	log_format = "json"
	new_base = "new base value"
	
	[p2p]
	external_address = "override.example.com"
	external-address = "override.example.com"
	seeds = "override@seed"
	new_field = "p2p"
	
	[rpc]
	cors_allowed_origins = ["override"]
	cors-allowed-origins = ["override"]
	
	[new_section]
	test = "value"
	
	[tx_index]
	indexer = "null"
	`)

			peers := Peers{
				client.ObjectKey{Name: "osmosis-0", Namespace: namespace}: {ExternalAddress: "should not see me"},
			}

			nodeKeys, err := getMockNodeKeysForCRD(*overrides, "")
			require.NoError(t, err)

			cms, err := BuildConfigMaps(overrides, peers, nodeKeys)
			require.NoError(t, err)

			cm := cms[0].Object()

			var (
				got  map[string]any
				want map[string]any
			)
			_, err = toml.Decode(wantCometOverrides, &want)
			require.NoError(t, err)

			_, err = toml.Decode(cm.Data["config-overlay.toml"], &got)
			require.NoError(t, err)

			var gotBuffer bytes.Buffer
			var wantBuffer bytes.Buffer

			require.NoError(t, toml.NewEncoder(&gotBuffer).Encode(got))
			require.NoError(t, toml.NewEncoder(&wantBuffer).Encode(want))

			fmt.Printf("got:\n%s\n", gotBuffer.String())
			fmt.Printf("want:\n%s\n", wantBuffer.String())

			require.Equal(t, want, got)
		})

		t.Run("p2p external addresses", func(t *testing.T) {
			peers := Peers{
				client.ObjectKey{Name: "osmosis-0", Namespace: namespace}: {ExternalAddress: "1.1.1.1:26657"},
				client.ObjectKey{Name: "osmosis-1", Namespace: namespace}: {ExternalAddress: "2.2.2.2:26657"},
			}
			p2pCrd := crd.DeepCopy()
			p2pCrd.Namespace = namespace
			p2pCrd.Spec.Replicas = 3

			nodeKeys, err := getMockNodeKeysForCRD(*p2pCrd, "")
			require.NoError(t, err)

			cms, err := BuildConfigMaps(p2pCrd, peers, nodeKeys)
			require.NoError(t, err)

			require.Equal(t, 3, len(cms))

			var decoded decodedToml
			_, err = toml.Decode(cms[0].Object().Data["config-overlay.toml"], &decoded)
			require.NoError(t, err)
			require.Equal(t, "1.1.1.1:26657", decoded["p2p"].(decodedToml)["external_address"])

			_, err = toml.Decode(cms[1].Object().Data["config-overlay.toml"], &decoded)
			require.NoError(t, err)
			require.Equal(t, "2.2.2.2:26657", decoded["p2p"].(decodedToml)["external_address"])

			_, err = toml.Decode(cms[2].Object().Data["config-overlay.toml"], &decoded)
			require.NoError(t, err)
			require.Empty(t, decoded["p2p"].(decodedToml)["external_address"])
		})

		t.Run("invalid toml", func(t *testing.T) {
			malformed := crd.DeepCopy()
			malformed.Spec.ChainSpec.Comet.TomlOverrides = ptr(`invalid_toml = should be invalid`)

			nodeKeys, err := getMockNodeKeysForCRD(*malformed, "")
			require.NoError(t, err)

			_, err = BuildConfigMaps(malformed, nil, nodeKeys)

			require.Error(t, err)
			require.Contains(t, err.Error(), "invalid toml in comet overrides")
		})
	})

	t.Run("app-overlay.toml", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Spec.ChainSpec.App = cosmosv1.SDKAppConfig{
			MinGasPrice: "0.123token",
		}

		t.Run("happy path", func(t *testing.T) {
			custom := crd.DeepCopy()
			custom.Spec.ChainSpec.App.APIEnableUnsafeCORS = true
			custom.Spec.ChainSpec.App.GRPCWebEnableUnsafeCORS = true
			custom.Spec.ChainSpec.App.HaltHeight = ptr(uint64(34567))
			custom.Spec.ChainSpec.App.Pruning = &cosmosv1.Pruning{
				Strategy:        "custom",
				Interval:        ptr(uint32(222)),
				KeepEvery:       ptr(uint32(333)),
				KeepRecent:      ptr(uint32(444)),
				MinRetainBlocks: ptr(uint32(271500)),
			}

			nodeKeys, err := getMockNodeKeysForCRD(*custom, "")
			require.NoError(t, err)

			cms, err := BuildConfigMaps(custom, nil, nodeKeys)
			require.NoError(t, err)

			cm := cms[0].Object()

			require.NotEmpty(t, cm.Data)
			require.Empty(t, cm.BinaryData)

			var (
				got  map[string]any
				want map[string]any
			)
			_, err = toml.Decode(wantApp, &want)
			require.NoError(t, err)

			_, err = toml.Decode(cm.Data["app-overlay.toml"], &got)
			require.NoError(t, err)

			require.Equal(t, want, got)
		})

		t.Run("defaults", func(t *testing.T) {
			nodeKeys, err := getMockNodeKeysForCRD(crd, "")
			require.NoError(t, err)

			cms, err := BuildConfigMaps(&crd, nil, nodeKeys)
			require.NoError(t, err)

			cm := cms[0].Object()

			var (
				got  map[string]any
				want map[string]any
			)
			_, err = toml.Decode(wantAppDefaults, &want)
			require.NoError(t, err)

			_, err = toml.Decode(cm.Data["app-overlay.toml"], &got)
			require.NoError(t, err)

			require.Equal(t, want, got)
		})

		t.Run("overrides", func(t *testing.T) {
			overrides := crd.DeepCopy()
			overrides.Spec.ChainSpec.App.MinGasPrice = "should not see me"
			overrides.Spec.ChainSpec.App.TomlOverrides = ptr(`
	minimum-gas-prices = "0.1override"
	new-base = "new base value"
	
	[api]
	enable = false
	new-field = "test"
	`)
			nodeKeys, err := getMockNodeKeysForCRD(*overrides, "")
			require.NoError(t, err)

			cms, err := BuildConfigMaps(overrides, nil, nodeKeys)
			require.NoError(t, err)

			cm := cms[0].Object()

			var (
				got  map[string]any
				want map[string]any
			)
			_, err = toml.Decode(wantAppOverrides, &want)
			require.NoError(t, err)

			_, err = toml.Decode(cm.Data["app-overlay.toml"], &got)
			require.NoError(t, err)

			require.Equal(t, want, got)
		})

		t.Run("external address overrides", func(t *testing.T) {
			overrides := crd.DeepCopy()

			overrides.Spec.InstanceOverrides = make(map[string]cosmosv1.InstanceOverridesSpec)
			overrideAddr0 := "override0.example.com:26656"
			overrideAddr1 := "override1.example.com:26656"
			overrides.Spec.InstanceOverrides["osmosis-0"] = cosmosv1.InstanceOverridesSpec{
				ExternalAddress: &overrideAddr0,
			}
			overrides.Spec.InstanceOverrides["osmosis-1"] = cosmosv1.InstanceOverridesSpec{
				ExternalAddress: &overrideAddr1,
			}

			nodeKeys, err := getMockNodeKeysForCRD(*overrides, "")
			require.NoError(t, err)

			cms, err := BuildConfigMaps(overrides, nil, nodeKeys)
			require.NoError(t, err)

			var config map[string]any

			_, err = toml.Decode(cms[0].Object().Data["config-overlay.toml"], &config)
			require.NoError(t, err)
			require.Equal(t, overrideAddr0, config["p2p"].(map[string]any)["external_address"])
			require.Equal(t, overrideAddr0, config["p2p"].(map[string]any)["external-address"])

			_, err = toml.Decode(cms[1].Object().Data["config-overlay.toml"], &config)
			require.NoError(t, err)
			require.Equal(t, overrideAddr1, config["p2p"].(map[string]any)["external_address"])
			require.Equal(t, overrideAddr1, config["p2p"].(map[string]any)["external-address"])
		})

		t.Run("invalid toml", func(t *testing.T) {
			malformed := crd.DeepCopy()
			malformed.Spec.ChainSpec.App.TomlOverrides = ptr(`invalid_toml = should be invalid`)

			nodeKeys, err := getMockNodeKeysForCRD(*malformed, "")
			require.NoError(t, err)

			_, err = BuildConfigMaps(malformed, nil, nodeKeys)

			require.Error(t, err)
			require.Contains(t, err.Error(), "invalid toml in app overrides")
		})
	})

	t.Run("node_key.json", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3

		t.Run("happy path", func(t *testing.T) {
			custom := crd.DeepCopy()

			nodeKeys, err := getMockNodeKeysForCRD(*custom, "")
			require.NoError(t, err)

			cms, err := BuildConfigMaps(custom, nil, nodeKeys)
			require.NoError(t, err)

			cm := cms[0].Object()

			require.NotEmpty(t, cm.Data)
			require.Empty(t, cm.BinaryData)

			nodeKey := NodeKey{}

			err = json.Unmarshal([]byte(cm.Data[nodeKeyFile]), &nodeKey)
			require.NoError(t, err)
			require.Equal(t, nodeKey.PrivKey.Type, "tendermint/PrivKeyEd25519")
			require.NotEmpty(t, nodeKey.PrivKey.Value)
		})

		t.Run("with existing", func(t *testing.T) {
			const namespace = "test-namespace"
			var crd cosmosv1.CosmosFullNode
			crd.Namespace = namespace
			crd.Name = "juno"
			crd.Spec.Replicas = 3

			var existingNodeKeys NodeKeys = map[client.ObjectKey]NodeKeyRepresenter{
				{Namespace: namespace, Name: "juno-0"}: {
					NodeKey: NodeKey{
						PrivKey: NodeKeyPrivKey{
							Type:  "tendermint/PrivKeyEd25519",
							Value: ed25519.PrivateKey{},
						},
					},
					MarshaledNodeKey: []byte("existing"),
				},
				{Namespace: namespace, Name: "juno-1"}: {
					NodeKey: NodeKey{
						PrivKey: NodeKeyPrivKey{
							Type:  "tendermint/PrivKeyEd25519",
							Value: ed25519.PrivateKey{},
						},
					},
					MarshaledNodeKey: []byte("existing"),
				},
				{Namespace: namespace, Name: "juno-2"}: {
					NodeKey: NodeKey{
						PrivKey: NodeKeyPrivKey{
							Type:  "tendermint/PrivKeyEd25519",
							Value: ed25519.PrivateKey{},
						},
					},
					MarshaledNodeKey: []byte("existing"),
				},
			}

			got, err := BuildConfigMaps(&crd, nil, existingNodeKeys)
			require.NoError(t, err)
			require.Equal(t, 3, len(got))

			nodeKey := got[0].Object().Data["node_key.json"]
			require.Equal(t, "existing", nodeKey)
		})
	})

	test.HasTypeLabel(t, func(crd cosmosv1.CosmosFullNode) []map[string]string {
		nodeKeys, err := getMockNodeKeysForCRD(crd, "")
		require.NoError(t, err)

		cms, _ := BuildConfigMaps(&crd, nil, nodeKeys)
		labels := make([]map[string]string, 0)
		for _, cm := range cms {
			labels = append(labels, cm.Object().Labels)
		}
		return labels
	})
}

func getNodeKeyFromConfigMap(cm *corev1.ConfigMap) (*NodeKeyRepresenter, error) {
	nodeKey := NodeKey{}

	nodeKeyData := []byte(cm.Data[nodeKeyFile])

	err := json.Unmarshal(nodeKeyData, &nodeKey)
	if err != nil {
		return nil, err
	}

	return &NodeKeyRepresenter{
		NodeKey:          nodeKey,
		MarshaledNodeKey: nodeKeyData,
	}, nil
}
