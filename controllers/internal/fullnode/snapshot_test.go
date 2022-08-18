package fullnode

import (
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
)

func TestSnapshotScript(t *testing.T) {
	t.Parallel()

	const (
		testURL         = "https://example.com/archive.tar"
		wantIfStatement = `ls "$DATA_DIR/*.db" 1> /dev/null 2>&1
if [ $? -eq 0 ]; then
	echo "Databases in $DATA_DIR already exists; skipping initialization."
	exit 0
fi`
	)
	t.Run("snapshot url", func(t *testing.T) {
		var cfg cosmosv1.CosmosChainConfig
		cfg.SnapshotURL = ptr(testURL)

		got := SnapshotScript(cfg)

		require.Contains(t, got, wantIfStatement)
		require.Contains(t, got, `SNAPSHOT_URL="https://example.com/archive.tar"`)
	})

	t.Run("snapshot script", func(t *testing.T) {
		var cfg cosmosv1.CosmosChainConfig
		cfg.SnapshotURL = ptr(testURL) // Asserts SnapshotScript takes precedence.
		cfg.SnapshotScript = ptr("echo hello")

		got := SnapshotScript(cfg)
		require.Contains(t, got, wantIfStatement)
		require.NotContains(t, got, testURL)
		require.Contains(t, got, "echo hello")
	})

	t.Run("zero state", func(t *testing.T) {
		var cfg cosmosv1.CosmosChainConfig
		require.Panics(t, func() {
			SnapshotScript(cfg)
		})
	})
}
