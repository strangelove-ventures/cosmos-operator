package fullnode

import (
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
)

func TestGenesisScript(t *testing.T) {
	requireValidScript := func(t *testing.T, script string) {
		t.Helper()
		require.NotEmpty(t, script)
		require.Contains(t, script, "#!/usr/bin/env sh")
		require.Contains(t, script, `if [ -f "$GENESIS_FILE" ]; then
	echo "Genesis file $GENESIS_FILE already exists; skipping initialization."
	exit 0
fi`)
	}

	t.Run("default", func(t *testing.T) {
		var cfg cosmosv1.CosmosChainConfig

		got := GenesisScript(cfg)
		requireValidScript(t, got)
		require.NotContains(t, got, "GENESIS_URL")
		require.Contains(t, got, `mv "$INIT_GENESIS_FILE" "$GENESIS_FILE"`)
	})

	t.Run("download", func(t *testing.T) {
		cfg := cosmosv1.CosmosChainConfig{
			GenesisURL: ptr("https://example.com/genesis.json"),
		}
		got := GenesisScript(cfg)
		requireValidScript(t, got)
		require.Contains(t, got, `GENESIS_URL="https://example.com/genesis.json"`)
		require.Contains(t, got, "download_json")
	})

	t.Run("custom", func(t *testing.T) {
		cfg := cosmosv1.CosmosChainConfig{
			// Keeping this to assert that custom script takes precedence.
			GenesisURL:    ptr("https://example.com/genesis.json"),
			GenesisScript: ptr("echo hi"),
		}
		got := GenesisScript(cfg)
		requireValidScript(t, got)

		require.NotContains(t, got, "GENESIS_URL")
		require.Contains(t, got, "echo hi")
	})
}
