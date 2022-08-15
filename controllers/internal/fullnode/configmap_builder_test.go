package fullnode

import (
	_ "embed"
	"testing"

	"github.com/BurntSushi/toml"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/tendermint1.toml
	wantTendermint1 string
	//go:embed testdata/tendermint_defaults.toml
	wantTendermintDefaults string
	//go:embed testdata/tendermint_overrides.toml
	wantTendermintOverrides string
)

func TestBuildConfigMap(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		base := defaultCRD()
		base.Name = "agoric"
		base.Namespace = "test"
		base.Spec.PodTemplate.Image = "agoric:v6.0.0"

		cm, err := BuildConfigMap(&base)
		require.NoError(t, err)

		require.Equal(t, "agoric-fullnode-config", cm.Name)
		require.Equal(t, "test", cm.Namespace)
		require.Nil(t, cm.Immutable)

		wantLabels := map[string]string{
			"app.kubernetes.io/created-by": "cosmosfullnode",
			"app.kubernetes.io/name":       "agoric-fullnode",
			"app.kubernetes.io/version":    "v6.0.0",
		}

		require.Equal(t, wantLabels, cm.Labels)
	})

	t.Run("config.toml", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.ChainConfig.Tendermint = cosmosv1.CosmosTendermintConfig{
			ExternalAddress:  "test.example.com",
			PersistentPeers:  "peer1@1.2.2.2:789,peer2@2.2.2.2:789,peer3@3.2.2.2:789",
			Seeds:            "seed1@1.1.1.1:456,seed2@1.1.1.1:456",
			MaxInboundPeers:  5,
			MaxOutboundPeers: 15,
		}

		t.Run("happy path", func(t *testing.T) {
			custom := crd.DeepCopy()

			custom.Spec.ChainConfig.Tendermint.LogLevel = ptr("debug")
			custom.Spec.ChainConfig.Tendermint.LogFormat = ptr("json")
			custom.Spec.ChainConfig.Tendermint.CorsAllowedOrigins = []string{"*"}

			cm, err := BuildConfigMap(custom)
			require.NoError(t, err)

			require.NotEmpty(t, cm.Data)
			require.Empty(t, cm.BinaryData)

			var (
				got  map[string]any
				want map[string]any
			)
			_, err = toml.Decode(wantTendermint1, &want)
			require.NoError(t, err)

			_, err = toml.Decode(cm.Data["config.toml"], &got)
			require.NoError(t, err)

			require.Equal(t, want, got)
		})

		t.Run("defaults", func(t *testing.T) {
			cm, err := BuildConfigMap(&crd)
			require.NoError(t, err)

			var (
				got  map[string]any
				want map[string]any
			)
			_, err = toml.Decode(wantTendermintDefaults, &want)
			require.NoError(t, err)

			_, err = toml.Decode(cm.Data["config.toml"], &got)
			require.NoError(t, err)

			require.Equal(t, want, got)
		})

		t.Run("overrides", func(t *testing.T) {
			overrides := crd.DeepCopy()
			overrides.Spec.ChainConfig.Tendermint.CorsAllowedOrigins = []string{"should not see me"}
			overrides.Spec.ChainConfig.Tendermint.TomlOverrides = ptr(`
log_format = "json"
new_base = "new base value"

[p2p]
seeds = "override@seed"
new_field = "p2p"

[rpc]
cors_allowed_origins = ["override"]

[new_section]
test = "value"

[tx_index]
indexer = "null"
`)

			cm, err := BuildConfigMap(overrides)
			require.NoError(t, err)

			var (
				got  map[string]any
				want map[string]any
			)
			_, err = toml.Decode(wantTendermintOverrides, &want)
			require.NoError(t, err)

			_, err = toml.Decode(cm.Data["config.toml"], &got)
			require.NoError(t, err)

			require.Equal(t, want, got)
		})

		t.Run("invalid toml", func(t *testing.T) {
			malformed := crd.DeepCopy()
			malformed.Spec.ChainConfig.Tendermint.TomlOverrides = ptr(`invalid_toml = should be invalid`)
			_, err := BuildConfigMap(malformed)

			require.Error(t, err)
			require.Contains(t, err.Error(), "invalid toml in overrides")
		})
	})

	t.Run("app.toml", func(t *testing.T) {
		t.Skip("TODO")
	})
}
