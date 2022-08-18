package fullnode

import (
	_ "embed"

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
	return ""
	//var scriptBody string
	//switch {
	//case cfg.GenesisScript != nil:
	//	scriptBody = *cfg.GenesisScript
	//case cfg.GenesisURL != nil:
	//	scriptBody = fmt.Sprintf("GENESIS_URL=%q\n%s", *cfg.GenesisURL, scriptDownloadGenesis)
	//default:
	//	scriptBody = scriptUseInitGenesis
	//}
	//
	//return fmt.Sprintf(genesisScriptWrapper, scriptBody)
}
