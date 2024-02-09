package fullnode

import (
	_ "embed"
	"errors"
	"fmt"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
)

var (
	//go:embed script/download-snapshot.sh
	scriptDownloadSnapshot string
)

// There are other files in the DATA_DIR that we must not touch, so we only test for the existence of database files.
const snapshotScriptWrapper = `set -eu
if test -n "$(find $DATA_DIR -maxdepth 1 -name '*.db' -print -quit)"; then
	echo "Databases in $DATA_DIR already exists; skipping initialization."
	exit 0
fi

%s

echo "$DATA_DIR initialized."
`

// DownloadSnapshotCommand returns a command and args for downloading and restoring from a snapshot.
func DownloadSnapshotCommand(cfg cosmosv1.ChainSpec) (string, []string) {
	args := []string{"-c"}
	switch {
	case cfg.CosmosSDK.SnapshotScript != nil:
		args = append(args, fmt.Sprintf(snapshotScriptWrapper, *cfg.CosmosSDK.SnapshotScript))
	case cfg.CosmosSDK.SnapshotURL != nil:
		args = append(args, fmt.Sprintf(snapshotScriptWrapper, scriptDownloadSnapshot), "-s", *cfg.CosmosSDK.SnapshotURL)
	default:
		panic(errors.New("attempted to restore from a snapshot but snapshots are not configured"))
	}

	return "sh", args
}
