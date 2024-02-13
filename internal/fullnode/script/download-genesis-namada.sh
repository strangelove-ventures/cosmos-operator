NAMADA_NETWORK_CONFIGS_SERVER="https://github.com/anoma/namada-shielded-expedition/releases/download/$CHAIN_ID"

if [ ! -f $CHAIN_HOME/$CHAIN_ID/validity-predicates.toml ]; then
    echo "Directory $CHAIN_ID does not exist. Downloading..."
    namada --base-dir $CHAIN_HOME client utils join-network --chain-id "$CHAIN_ID"
    cp $CHAIN_HOME/$CHAIN_ID/config.toml $CHAIN_HOME/$CHAIN_ID/default-config.toml
    echo "$CHAIN_ID downloaded successfully."
else
    echo "Directory $CHAIN_ID already exists."
fi
