set -eu


CHAIN_ID=$1
NAMADA_NETWORK_CONFIGS_SERVER="https://github.com/anoma/namada-shielded-expedition/releases/download/$CHAIN_ID"
NAMADA_NETWORK_CONFIGS_SERVER=$2
namada --base-dir /data/namada client utils join-network --chain-id $CHAIN_ID