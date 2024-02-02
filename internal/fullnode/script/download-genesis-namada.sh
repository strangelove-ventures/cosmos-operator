set -eu


CHAIN_ID=$0
NAMADA_NETWORK_CONFIGS_SERVER="https://github.com/anoma/namada-shielded-expedition/releases/download/$CHAIN_ID"
NAMADA_NETWORK_CONFIGS_SERVER=$1
namada --base-dir $CHAIN_HOME client utils join-network --chain-id $CHAIN_ID