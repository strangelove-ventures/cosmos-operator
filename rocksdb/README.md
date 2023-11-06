# RocksDB Static Build

This Dockerfile produces cross-architecture (amd64 and arm64) docker images with a static rocksdb library.

## Reason

This static rocksdb build takes a while, and it is not necessary to build every time the cosmos-operator docker image is built, so this image caches the required artifacts to link rocksdb into the operator build.

## Build and push to Github Container Registry

```
ROCKSDB_VERSION=v7.10.2
docker buildx build --platform linux/arm64,linux/amd64 --build-arg "ROCKSDB_VERSION=$ROCKSDB_VERSION" --push -t ghcr.io/strangelove-ventures/rocksdb:$ROCKSDB_VERSION .
```

After publishing a new version, import that version in the `Dockerfile` and `local.Dockerfile` in the root of the cosmos-operator repository