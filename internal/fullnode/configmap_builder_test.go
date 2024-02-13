package fullnode

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	"github.com/bharvest-devops/cosmos-operator/internal/test"
	"github.com/stretchr/testify/require"
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

		tomlOverrides := `moniker = "agoric"`
		crd.Spec.ChainSpec.Comet.TomlOverrides = &tomlOverrides

		cms, err := BuildConfigMaps(&crd, nil)
		require.NoError(t, err)
		require.Equal(t, 3, len(cms))

		require.Equal(t, int64(0), cms[0].Ordinal())
		require.NotEmpty(t, cms[0].Revision())

		cm := cms[0].Object()
		require.Equal(t, "agoric-0", cm.Name)
		require.Equal(t, "test", cm.Namespace)
		require.Nil(t, cm.Immutable)

		wantLabels := map[string]string{
			"app.kubernetes.io/created-by": "cosmos-operator",
			"app.kubernetes.io/component":  "CosmosFullNode",
			"app.kubernetes.io/name":       "agoric",
			"app.kubernetes.io/instance":   "agoric-0",
			"app.kubernetes.io/version":    "v6.0.0",
			"cosmos.bharvest/network":      "testnet",
			"cosmos.bharvest/type":         "FullNode",
		}
		require.Empty(t, cm.Annotations)

		require.Equal(t, wantLabels, cm.Labels)

		cm = cms[1].Object()
		require.Equal(t, "agoric-1", cm.Name)

		require.NotEmpty(t, cms[0].Object().Data)
		require.Equal(t, cms[0].Object().Data, cms[1].Object().Data)

		crd.Spec.Type = cosmosv1.FullNode
		cms2, err := BuildConfigMaps(&crd, nil)

		require.NoError(t, err)
		require.Equal(t, cms, cms2)
	})

	t.Run("long name", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Name = strings.Repeat("chain", 300)
		crd.Spec.ChainSpec.Network = strings.Repeat("network", 300)

		cms, err := BuildConfigMaps(&crd, nil)
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

		persistentPeers := "peer1@1.2.2.2:789,peer2@2.2.2.2:789,peer3@3.2.2.2:789"
		seeds := "seed1@1.1.1.1:456,seed2@1.1.1.1:456"

		cosmosP2P := cosmosv1.P2P{
			PersistentPeers: &persistentPeers,
			Seeds:           &seeds,
		}
		cometConfig := cosmosv1.CometConfig{
			P2P: &cosmosP2P,
		}
		crd.Spec.ChainSpec.Comet = &cometConfig

		cosmosAppConfig := cosmosv1.SDKAppConfig{}
		crd.Spec.ChainSpec.CosmosSDK = &cosmosAppConfig

		t.Run("happy path", func(t *testing.T) {
			custom := crd.DeepCopy()
			custom.Spec.Replicas = 1
			custom.Spec.ChainSpec.LogLevel = ptr("debug")
			custom.Spec.ChainSpec.LogFormat = ptr("json")
			corsAllowedOrigins := []string{"*"}

			RPC := cosmosv1.RPC{
				CorsAllowedOrigins: &corsAllowedOrigins,
			}
			custom.Spec.ChainSpec.Comet.RPC = &RPC

			crd.Spec.ChainSpec.Comet.P2P.MaxNumInboundPeers = ptr(int32(5))
			crd.Spec.ChainSpec.Comet.P2P.MaxNumOutboundPeers = ptr(int32(15))

			peers := Peers{
				client.ObjectKey{Namespace: namespace, Name: "osmosis-0"}: {NodeID: "should not see me", PrivateAddress: "should not see me"},
			}
			cms, err := BuildConfigMaps(custom, peers)
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
			cms, err := BuildConfigMaps(&crd, nil)
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

			privatePeerIds := "private1,private2"
			unconditoinalPeerIDs := "unconditional1,unconditional2"
			peerCRD.Spec.ChainSpec.Comet.P2P.UnconditionalPeerIDs = &unconditoinalPeerIDs
			peerCRD.Spec.ChainSpec.Comet.P2P.PrivatePeerIds = &privatePeerIds
			peers := Peers{
				client.ObjectKey{Namespace: namespace, Name: "osmosis-0"}: {NodeID: "0", PrivateAddress: "0.local:26656"},
				client.ObjectKey{Namespace: namespace, Name: "osmosis-1"}: {NodeID: "1", PrivateAddress: "1.local:26656"},
				client.ObjectKey{Namespace: namespace, Name: "osmosis-2"}: {NodeID: "2", PrivateAddress: "2.local:26656"},
			}
			cms, err := BuildConfigMaps(peerCRD, peers)
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
			cms, err := BuildConfigMaps(sentry, nil)
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

			RPC := cosmosv1.RPC{
				CorsAllowedOrigins: ptr([]string{"should not see me"}),
			}
			overrides.Spec.ChainSpec.Comet.RPC = &RPC

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
			cms, err := BuildConfigMaps(overrides, peers)
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
			cms, err := BuildConfigMaps(p2pCrd, peers)
			require.NoError(t, err)

			require.Equal(t, 3, len(cms))

			var decoded cosmosv1.CometConfig
			_, err = toml.Decode(cms[0].Object().Data["config-overlay.toml"], &decoded)
			require.NoError(t, err)
			require.Equal(t, "1.1.1.1:26657", *decoded.P2P.ExternalAddress)
			decoded = cosmosv1.CometConfig{}

			_, err = toml.Decode(cms[1].Object().Data["config-overlay.toml"], &decoded)
			require.NoError(t, err)
			require.Equal(t, "2.2.2.2:26657", *decoded.P2P.ExternalAddress)

			empty := ""
			tmpP2P := cosmosv1.P2P{
				ExternalAddress: &empty,
			}
			decoded = cosmosv1.CometConfig{
				P2P: &tmpP2P,
			}

			_, err = toml.Decode(cms[2].Object().Data["config-overlay.toml"], &decoded)
			require.NoError(t, err)
			require.Empty(t, *decoded.P2P.ExternalAddress)
		})

		t.Run("invalid toml", func(t *testing.T) {
			malformed := crd.DeepCopy()
			malformed.Spec.ChainSpec.Comet.TomlOverrides = ptr(`invalid_toml = should be invalid`)
			_, err := BuildConfigMaps(malformed, nil)

			require.Error(t, err)
			require.Contains(t, err.Error(), "toml task failed")
		})
	})

	t.Run("app-overlay.toml", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		appConfig := cosmosv1.SDKAppConfig{
			MinGasPrice: "0.123token",
		}
		crd.Spec.ChainSpec.CosmosSDK = &appConfig

		t.Run("happy path", func(t *testing.T) {
			custom := crd.DeepCopy()
			custom.Spec.ChainSpec.CosmosSDK.APIEnableUnsafeCORS = true
			custom.Spec.ChainSpec.CosmosSDK.GRPCWebEnableUnsafeCORS = true
			custom.Spec.ChainSpec.CosmosSDK.HaltHeight = ptr(uint64(34567))
			custom.Spec.ChainSpec.CosmosSDK.Pruning = &cosmosv1.Pruning{
				Strategy:        "custom",
				Interval:        ptr(uint32(222)),
				KeepEvery:       ptr(uint32(333)),
				KeepRecent:      ptr(uint32(444)),
				MinRetainBlocks: ptr(uint32(271500)),
			}

			cms, err := BuildConfigMaps(custom, nil)
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
			cms, err := BuildConfigMaps(&crd, nil)
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
			overrides.Spec.ChainSpec.CosmosSDK.MinGasPrice = "should not see me"
			overrides.Spec.ChainSpec.CosmosSDK.TomlOverrides = ptr(`
	minimum-gas-prices = "0.1override"
	new-base = "new base value"
	
	[api]
	enable = false
	new-field = "test"
	`)
			cms, err := BuildConfigMaps(overrides, nil)
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
			cms, err := BuildConfigMaps(overrides, nil)
			require.NoError(t, err)

			var config map[string]any

			_, err = toml.Decode(cms[0].Object().Data["config-overlay.toml"], &config)
			require.NoError(t, err)
			require.Equal(t, overrideAddr0, config["p2p"].(map[string]any)["external_address"])

			_, err = toml.Decode(cms[1].Object().Data["config-overlay.toml"], &config)
			require.NoError(t, err)
			require.Equal(t, overrideAddr1, config["p2p"].(map[string]any)["external_address"])
		})

		t.Run("invalid toml", func(t *testing.T) {
			malformed := crd.DeepCopy()
			malformed.Spec.ChainSpec.CosmosSDK.TomlOverrides = ptr(`invalid_toml = should be invalid`)
			_, err := BuildConfigMaps(malformed, nil)

			require.Error(t, err)
			require.Contains(t, err.Error(), "toml task failed")
		})
	})

	test.HasTypeLabel(t, func(crd cosmosv1.CosmosFullNode) []map[string]string {
		cometConfig := cosmosv1.CometConfig{}
		crd.Spec.ChainSpec.Comet = &cometConfig
		cosmosAppConfig := cosmosv1.SDKAppConfig{}
		crd.Spec.ChainSpec.CosmosSDK = &cosmosAppConfig

		cms, _ := BuildConfigMaps(&crd, nil)
		labels := make([]map[string]string, 0)
		for _, cm := range cms {
			labels = append(labels, cm.Object().Labels)
		}
		return labels
	})
}
