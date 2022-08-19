set -eu

# $GENESIS_FILE and $CONFIG_DIR already set via pod env vars.

INIT_GENESIS_FILE="$HOME/.tmp/config/genesis.json"

echo "Using initialized genesis file $INIT_GENESIS_FILE..."

set -x

mv "$INIT_GENESIS_FILE" "$GENESIS_FILE"

set +x

echo "Move complete."
