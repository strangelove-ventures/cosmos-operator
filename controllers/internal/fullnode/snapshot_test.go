package fullnode

import (
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
)

func TestDownloadSnapshotCommand(t *testing.T) {
	t.Parallel()

	const (
		testURL         = "https://example.com/archive.tar"
		wantIfStatement = `if test -n "$(find $DATA_DIR -maxdepth 1 -name '*.db' -print -quit)"; then
	echo "Databases in $DATA_DIR already exists; skipping initialization."
	exit 0
fi`
	)
	t.Run("snapshot url", func(t *testing.T) {
		var cfg cosmosv1.CosmosChainConfig
		cfg.SnapshotURL = ptr(testURL)

		cmd, args := DownloadSnapshotCommand(cfg)
		require.Equal(t, "sh", cmd)
		require.Equal(t, "-c", args[0])

		script := args[1]
		require.Contains(t, script, wantIfStatement)
		require.Contains(t, script, `SNAPSHOT_URL="https://example.com/archive.tar"`)
	})

	t.Run("snapshot script", func(t *testing.T) {
		var cfg cosmosv1.CosmosChainConfig
		cfg.SnapshotURL = ptr(testURL) // Asserts SnapshotScript takes precedence.
		cfg.SnapshotScript = ptr("echo hello")

		_, args := DownloadSnapshotCommand(cfg)
		got := args[1]
		require.Contains(t, got, wantIfStatement)
		require.NotContains(t, got, testURL)
		require.Contains(t, got, "echo hello")
	})

	t.Run("zero state", func(t *testing.T) {
		var cfg cosmosv1.CosmosChainConfig
		require.Panics(t, func() {
			DownloadSnapshotCommand(cfg)
		})
	})
}
