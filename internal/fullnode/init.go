package fullnode

import (
	_ "embed"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
)

const initScriptWrapper = `
set -eu

echo CHAIN_HOME: $CHAIN_HOME

if [ ! -d "$CHAIN_HOME/.penumbra" ]; then
	echo "Initializing chain..."
	%s
else
	echo "Skipping chain init; already initialized."
fi
echo "Done"
`

const cosmosInitScriptWrapper = `
set -eu
if [ ! -d "$CHAIN_HOME/data" ]; then
	echo "Initializing chain..."
	%s --home "$CHAIN_HOME"
else
	echo "Skipping chain init; already initialized."
fi

echo "Initializing into tmp dir for downstream processing..."
%s --home "$HOME/.tmp"`

func InitCommand(chainSpec cosmosv1.ChainSpec, moniker string) (string, []string) {
	args := []string{"-c"}
	switch {
	case chainSpec.InitScript != nil:
		args = append(args, fmt.Sprintf(initScriptWrapper, *chainSpec.InitScript))
	default:
		initScript := fmt.Sprintf("%s init --chain-id %s %s", chainSpec.Binary, chainSpec.ChainID, moniker)
		args = append(args, fmt.Sprintf(cosmosInitScriptWrapper, initScript, initScript))
	}

	return "sh", args
}
