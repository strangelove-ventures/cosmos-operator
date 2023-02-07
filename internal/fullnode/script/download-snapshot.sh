set -eu

# $CHAIN_HOME already set via pod env vars.

SNAPSHOT_URL="$1"

echo "Downloading snapshot archive $SNAPSHOT_URL to $CHAIN_HOME..."

download_tar() {
  echo "Downloading and extracting tar..."
  wget -c -O - "$SNAPSHOT_URL" | tar -x -C "$CHAIN_HOME"
}

download_targz() {
  echo "Downloading and extracting compressed tar..."
  wget -c -O - "$SNAPSHOT_URL" | tar -xz -C "$CHAIN_HOME"
}

download_lz4() {
  echo "Downloading and extracting lz4..."
  wget -c -O - "$SNAPSHOT_URL" | lz4 -c -d | tar -x -C "$CHAIN_HOME"
}

case "$SNAPSHOT_URL" in
*.tar.lz4) download_lz4 ;;
*.tar.gzip) download_targz ;;
*.tar.gz) download_targz ;;
*.tar) download_tar ;;
*)
  echo "Unable to handle file extension for $SNAPSHOT_URL"
  exit 1
  ;;
esac

echo "Download and extract snapshot complete."
