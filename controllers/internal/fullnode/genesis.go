package fullnode

import (
	_ "embed"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
)

var (
	//go:embed script/download-genesis.sh
	scriptDownloadGenesis string
	//go:embed script/use-init-genesis.sh
	scriptUseInitGenesis string
)

const scriptWrapper = `if [ -f "$GENESIS_FILE" ]; then
	echo "Genesis file $GENESIS_FILE already exists; skipping initialization."
	exit 0
fi

%s

echo "Genesis $GENESIS_FILE initialized."
`

// GenesisScript returns a proper genesis script for use in an init container.
//
// The general strategy is if the user does not configure an external genesis file, use the genesis from the <chain-binary> init command.
// If the user supplies a custom script, we use that. Otherwise, we use attempt to download and extract the file.
func GenesisScript(cfg cosmosv1.CosmosChainConfig) string {
	var scriptBody string
	switch {
	case cfg.GenesisScript != nil:
		scriptBody = *cfg.GenesisScript
	case cfg.GenesisURL != nil:
		scriptBody = fmt.Sprintf("GENESIS_URL=%q\n%s", *cfg.GenesisURL, scriptDownloadGenesis)
	default:
		scriptBody = scriptUseInitGenesis
	}

	return fmt.Sprintf(scriptWrapper, scriptBody)
}
