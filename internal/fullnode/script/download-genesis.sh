set -eu

# $GENESIS_FILE and $CONFIG_DIR already set via pod env vars.

GENESIS_URL="$1"

echo "Downloading genesis file $GENESIS_URL to $GENESIS_FILE..."

download_json() {
  echo "Downloading plain json..."
  wget -c -O "$GENESIS_FILE" "$GENESIS_URL"
}

download_jsongz() {
  echo "Downloading json.gz..."
  wget -c -O - "$GENESIS_URL" | gunzip -c >"$GENESIS_FILE"
}

download_tar() {
  echo "Downloading and extracting tar..."
  wget -c -O - "$GENESIS_URL" | tar -x -C "$CONFIG_DIR"
}

download_targz() {
  echo "Downloading and extracting compressed tar..."
  wget -c -O - "$GENESIS_URL" | tar -xz -C "$CONFIG_DIR"
}

download_zip() {
  echo "Downloading and extracting zip..."
  wget -c -O tmp_genesis.zip "$GENESIS_URL"
  unzip tmp_genesis.zip
  rm tmp_genesis.zip
  mv genesis.json "$GENESIS_FILE"
}

rm -f "$GENESIS_FILE"

case "$GENESIS_URL" in
*.json.gz) download_jsongz ;;
*.json) download_json ;;
*.tar.gz) download_targz ;;
*.tar.gzip) download_targz ;;
*.tar) download_tar ;;
*.zip) download_zip ;;
*)
  echo "Unable to handle file extension for $GENESIS_URL"
  exit 1
  ;;
esac

echo "Saved genesis file to $GENESIS_FILE."
echo "Download genesis file complete."
