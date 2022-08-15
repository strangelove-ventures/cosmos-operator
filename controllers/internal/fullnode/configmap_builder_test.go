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
)

func TestBuildConfigMap(t *testing.T) {
	tendermint := cosmosv1.CosmosTendermintConfig{
		ExternalAddress:    "test.example.com",
		PersistentPeers:    []string{"peer1@1.2.2.2:789", "peer2@2.2.2.2:789", "peer3@3.2.2.2:789"},
		Seeds:              []string{"seed1@1.1.1.1:456", "seed2@1.1.1.1:456"},
		MaxInboundPeers:    5,
		MaxOutboundPeers:   15,
		CorsAllowedOrigins: []string{"*"},
		LogLevel:           ptr("debug"),
		LogFormat:          ptr("json"),
	}
	// TODO
	app := cosmosv1.CosmosAppConfig{}

	t.Run("happy path", func(t *testing.T) {
		cm, kerr := BuildConfigMap(tendermint, app)
		require.NoError(t, kerr)

		require.NotEmpty(t, cm.Data)
		require.Empty(t, cm.BinaryData)

		var (
			got  map[string]any
			want map[string]any
		)
		_, err := toml.Decode(wantTendermint1, &want)
		require.NoError(t, err)

		_, err = toml.Decode(cm.Data["config.toml"], &got)
		require.NoError(t, err)

		require.Equal(t, want, got)
	})

	t.Run("defaults", func(t *testing.T) {
		defaults := tendermint.DeepCopy()
		defaults.LogLevel = nil
		defaults.LogFormat = nil
		defaults.CorsAllowedOrigins = nil

		cm, kerr := BuildConfigMap(*defaults, app)
		require.NoError(t, kerr)

		var (
			got  map[string]any
			want map[string]any
		)
		_, err := toml.Decode(wantTendermintDefaults, &want)
		require.NoError(t, err)

		_, err = toml.Decode(cm.Data["config.toml"], &got)
		require.NoError(t, err)

		require.Equal(t, want, got)
	})

	t.Run("overrides", func(t *testing.T) {
		t.Fatal("todo")
	})

	t.Run("invalid toml", func(t *testing.T) {
		t.Fatal("TODO non recoverable")
	})
}
