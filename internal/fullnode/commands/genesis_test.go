package commands

import (
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
)

func TestDownloadGenesisCommand(t *testing.T) {
	t.Parallel()

	requireValidScript := func(t *testing.T, script string) {
		t.Helper()
		require.NotEmpty(t, script)
		require.Contains(t, script, `if [ $DB_INIT -eq 0 ]`)
	}

	t.Run("default", func(t *testing.T) {
		var cfg cosmosv1.ChainSpec

		cmd, args := DownloadGenesisCommand(cfg)
		require.Equal(t, "sh", cmd)

		require.Len(t, args, 2)

		require.Equal(t, "-c", args[0])

		got := args[1]
		requireValidScript(t, got)
		require.NotContains(t, got, "GENESIS_URL")
		require.Contains(t, got, `mv "$INIT_GENESIS_FILE" "$GENESIS_FILE"`)
	})

	t.Run("download", func(t *testing.T) {
		cfg := cosmosv1.ChainSpec{
			GenesisURL: ptr("https://example.com/genesis.json"),
		}
		cmd, args := DownloadGenesisCommand(cfg)
		require.Equal(t, "sh", cmd)

		require.Len(t, args, 4)

		require.Equal(t, "-c", args[0])
		got := args[1]
		requireValidScript(t, got)
		require.Contains(t, got, `GENESIS_URL`)
		require.Contains(t, got, "download_json")

		require.Equal(t, "-s", args[2])
		require.Equal(t, "https://example.com/genesis.json", args[3])
	})

	t.Run("custom", func(t *testing.T) {
		cfg := cosmosv1.ChainSpec{
			// Keeping this to assert that custom script takes precedence.
			GenesisURL:    ptr("https://example.com/genesis.json"),
			GenesisScript: ptr("echo hi"),
		}
		cmd, args := DownloadGenesisCommand(cfg)
		require.Equal(t, "sh", cmd)

		require.Len(t, args, 2)

		require.Equal(t, "-c", args[0])

		got := args[1]
		requireValidScript(t, got)

		require.NotContains(t, got, "GENESIS_URL")
		require.Contains(t, got, "echo hi")
	})
}
