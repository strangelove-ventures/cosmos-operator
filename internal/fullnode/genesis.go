package fullnode

import (
	_ "embed"
	"fmt"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
)

var (
	//go:embed script/download-genesis.sh
	scriptDownloadGenesis string
	//go:embed script/use-init-genesis.sh
	scriptUseInitGenesis string
	//go:embed script/download-genesis-namada.sh
	scriptDownloadGenesisNamada string
)

// If $DATA_DIR is populated, then we assume we have the genesis file.
const genesisScriptWrapper = `ls $DATA_DIR/*.db 1> /dev/null 2>&1
DB_INIT=$?
if [ $DB_INIT -eq 0 ]; then
	echo "Database already initialized, skipping genesis initialization"
	exit 0
fi

%s

echo "Genesis $GENESIS_FILE initialized."
`

// DownloadGenesisCommand returns a proper genesis command for use in an init container.
//
// The general strategy is if the user does not configure an external genesis file, use the genesis from the <chain-binary> init command.
// If the user supplies a custom script, we use that. Otherwise, we use attempt to download and extract the file.
func DownloadGenesisCommand(cfg cosmosv1.ChainSpec) (string, []string) {
	args := []string{"-c"}
	switch {
	case cfg.ChainType == chainTypeNamada:
		args = append(args, scriptDownloadGenesisNamada, *cfg.GenesisURL)
	case cfg.GenesisScript != nil:
		args = append(args, fmt.Sprintf(genesisScriptWrapper, *cfg.GenesisScript))
	case cfg.GenesisURL != nil:
		args = append(args, fmt.Sprintf(genesisScriptWrapper, scriptDownloadGenesis), "-s", *cfg.GenesisURL)
	default:
		args = append(args, fmt.Sprintf(genesisScriptWrapper, scriptUseInitGenesis))
	}
	return "sh", args
}
