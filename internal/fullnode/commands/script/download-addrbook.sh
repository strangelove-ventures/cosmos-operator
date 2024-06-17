set -eu

# $ADDRBOOK_FILE and $CONFIG_DIR already set via pod env vars.

ADDRBOOK_URL="$1"

echo "Downloading address book file $ADDRBOOK_URL to $ADDRBOOK_FILE..."

download_json() {
  echo "Downloading plain json..."
  wget -c -O "$ADDRBOOK_FILE" "$ADDRBOOK_URL"
}

download_jsongz() {
  echo "Downloading json.gz..."
  wget -c -O - "$ADDRBOOK_URL" | gunzip -c >"$ADDRBOOK_FILE"
}

download_tar() {
  echo "Downloading and extracting tar..."
  wget -c -O - "$ADDRBOOK_URL" | tar -x -C "$CONFIG_DIR"
}

download_targz() {
  echo "Downloading and extracting compressed tar..."
  wget -c -O - "$ADDRBOOK_URL" | tar -xz -C "$CONFIG_DIR"
}

download_zip() {
  echo "Downloading and extracting zip..."
  wget -c -O tmp_genesis.zip "$ADDRBOOK_URL"
  unzip tmp_genesis.zip
  rm tmp_genesis.zip
  mv genesis.json "$ADDRBOOK_FILE"
}

rm -f "$ADDRBOOK_FILE"

case "$ADDRBOOK_URL" in
*.json.gz) download_jsongz ;;
*.json) download_json ;;
*.tar.gz) download_targz ;;
*.tar.gzip) download_targz ;;
*.tar) download_tar ;;
*.zip) download_zip ;;
*)
  echo "Unable to handle file extension for $ADDRBOOK_URL"
  exit 1
  ;;
esac

echo "Saved address book file to $ADDRBOOK_FILE."
echo "Download address book file complete."
