package fullnode

import (
	_ "embed"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
)

var (
	//go:embed script/download-addrbook.sh
	scriptDownloadAddrbook string
)

const addrbookScriptWrapper = `ls $CONFIG_DIR/addrbook.json 1> /dev/null 2>&1
ADDRBOOK_EXISTS=$?
if [ $ADDRBOOK_EXISTS -eq 0 ]; then
	echo "Address book already exists"
	exit 0
fi
ls -l $CONFIG_DIR/addrbook.json 
%s
ls -l $CONFIG_DIR/addrbook.json 

echo "Address book $ADDRBOOK_FILE downloaded"
`

// DownloadGenesisCommand returns a proper address book command for use in an init container.
func DownloadAddrbookCommand(cfg cosmosv1.ChainSpec) (string, []string) {
	args := []string{"-c"}
	switch {
	case cfg.AddrbookScript != nil:
		args = append(args, fmt.Sprintf(addrbookScriptWrapper, *cfg.AddrbookScript))
	case cfg.AddrbookURL != nil:
		args = append(args, fmt.Sprintf(addrbookScriptWrapper, scriptDownloadAddrbook), "-s", *cfg.AddrbookURL)
	default:
		args = append(args, "echo Using default address book")
	}
	return "sh", args
}
