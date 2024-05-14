package fullnode

import (
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
)

func TestInitCommand(t *testing.T) {
	t.Parallel()

	t.Run("given cosmos chain", func(t *testing.T) {
		t.Run("returns init command", func(t *testing.T) {
			var cfg = cosmosv1.ChainSpec{
				Binary:  "gaiad",
				ChainID: "cosmoshub-4",
			}

			cmd, args := InitCommand(cfg, "strangelove")
			require.Equal(t, "sh", cmd)

			require.Len(t, args, 2)

			require.Equal(t, "-c", args[0])

			got := args[1]
			require.Contains(t, got, "if [ ! -d \"$CHAIN_HOME/data\" ]; then")
			require.Contains(t, got, "gaiad init --chain-id cosmoshub-4 strangelove --home \"$CHAIN_HOME\"")
		})
	})

	t.Run("given custom chain", func(t *testing.T) {
		t.Run("returns init command", func(t *testing.T) {
			var initScript = "pd testnet --testnet-dir /home/operator/cosmos/.penumbra/testnet_data  join --external-address 127.0.0.1:26656 --moniker strangelove"
			var cfg = cosmosv1.ChainSpec{
				InitScript: &initScript,
			}

			cmd, args := InitCommand(cfg, "strangelove")
			require.Equal(t, "sh", cmd)

			require.Len(t, args, 2)

			require.Equal(t, "-c", args[0])

			got := args[1]
			require.Contains(t, got, "if [ ! -d \"$CHAIN_HOME/.penumbra\" ]; then")
			require.Contains(t, got, "pd testnet --testnet-dir /home/operator/cosmos/.penumbra/testnet_data  join --external-address 127.0.0.1:26656 --moniker strangelove")
		})
	})
}
