package fullnode

import (
	_ "embed"
	"errors"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
)

var (
	//go:embed script/download-snapshot.sh
	scriptDownloadSnapshot string
)

// There are other files in the DATA_DIR that we must not touch, so we only test for the existence of database files.
const snapshotScriptWrapper = `ls "$DATA_DIR/*.db" 1> /dev/null 2>&1
if [ $? -eq 0 ]; then
	echo "Databases in $DATA_DIR already exists; skipping initialization."
	exit 0
fi

%s

echo "$DATA_DIR initialized."
`

func SnapshotScript(cfg cosmosv1.CosmosChainConfig) string {
	var scriptBody string
	switch {
	case cfg.SnapshotScript != nil:
		scriptBody = *cfg.SnapshotScript
	case cfg.SnapshotURL != nil:
		scriptBody = fmt.Sprintf("SNAPSHOT_URL=%q\n%s", *cfg.SnapshotURL, scriptDownloadSnapshot)
	default:
		panic(errors.New("attempted to restore from a snapshot but snapshots not not configured"))
	}

	return fmt.Sprintf(snapshotScriptWrapper, scriptBody)
}
