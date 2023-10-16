package fullnode

import (
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
)

func TestDownloadAddrbookCommand(t *testing.T) {
	t.Parallel()

	requireValidScript := func(t *testing.T, script string) {
		t.Helper()
		require.NotEmpty(t, script)
		require.Contains(t, script, `if [ $ADDRBOOK_EXISTS -eq 0 ]`)
	}

	t.Run("default", func(t *testing.T) {
		var cfg cosmosv1.ChainSpec

		cmd, args := DownloadAddrbookCommand(cfg)
		require.Equal(t, "sh", cmd)

		require.Len(t, args, 2)

		require.Equal(t, "-c", args[0])

		got := args[1]
		require.NotContains(t, got, "ADDRBOOK_EXISTS")
		require.Contains(t, got, "Using default address book")
	})

	t.Run("download", func(t *testing.T) {
		cfg := cosmosv1.ChainSpec{
			AddrbookURL: ptr("https://example.com/addrbook.json"),
		}
		cmd, args := DownloadAddrbookCommand(cfg)
		require.Equal(t, "sh", cmd)

		require.Len(t, args, 4)

		require.Equal(t, "-c", args[0])
		got := args[1]
		requireValidScript(t, got)
		require.Contains(t, got, `ADDRBOOK_URL`)
		require.Contains(t, got, "download_json")

		require.Equal(t, "-s", args[2])
		require.Equal(t, "https://example.com/addrbook.json", args[3])
	})

	t.Run("custom", func(t *testing.T) {
		cfg := cosmosv1.ChainSpec{
			// Keeping this to assert that custom script takes precedence.
			AddrbookURL:    ptr("https://example.com/addrbook.json"),
			AddrbookScript: ptr("echo hi"),
		}
		cmd, args := DownloadAddrbookCommand(cfg)
		require.Equal(t, "sh", cmd)

		require.Len(t, args, 2)

		require.Equal(t, "-c", args[0])

		got := args[1]
		requireValidScript(t, got)

		require.NotContains(t, got, "ADDRBOOK_URL")
		require.Contains(t, got, "echo hi")
	})
}
