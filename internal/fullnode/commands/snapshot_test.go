package commands

import (
	"testing"

	"github.com/samber/lo"
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
		var cfg cosmosv1.ChainSpec
		cfg.SnapshotURL = lo.ToPtr(testURL)

		cmd, args := DownloadSnapshotCommand(cfg)
		require.Equal(t, "sh", cmd)

		require.Len(t, args, 4)

		require.Equal(t, "-c", args[0])

		script := args[1]
		require.Contains(t, script, wantIfStatement)
		require.Contains(t, script, `SNAPSHOT_URL`)

		require.Equal(t, "-s", args[2])
		require.Equal(t, testURL, args[3])
	})

	t.Run("snapshot script", func(t *testing.T) {
		var cfg cosmosv1.ChainSpec
		cfg.SnapshotURL = lo.ToPtr(testURL) // Asserts SnapshotScript takes precedence.
		cfg.SnapshotScript = lo.ToPtr("echo hello")

		_, args := DownloadSnapshotCommand(cfg)
		require.Len(t, args, 2)

		require.Equal(t, "-c", args[0])

		got := args[1]
		require.Contains(t, got, wantIfStatement)
		require.NotContains(t, got, "SNAPSHOT_URL")
		require.Contains(t, got, "echo hello")
	})

	t.Run("zero state", func(t *testing.T) {
		var cfg cosmosv1.ChainSpec
		require.Panics(t, func() {
			DownloadSnapshotCommand(cfg)
		})
	})
}
