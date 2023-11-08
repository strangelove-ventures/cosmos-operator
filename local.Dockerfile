# See rocksdb/README.md for instructions to update rocksdb version
FROM ghcr.io/strangelove-ventures/rocksdb:v7.10.2 AS rocksdb

FROM golang:1.20-alpine AS builder

RUN apk add --update --no-cache\
    gcc\
    libc-dev\
    git\
    make\
    bash\
    g++\
    linux-headers\
    perl\
    snappy-dev\
    zlib-dev\
    bzip2-dev\
    lz4-dev\
    zstd-dev\
    snappy-static\
    zlib-static\
    bzip2-static\
    lz4-static\
    zstd-static

# Install RocksDB headers and static library
COPY --from=rocksdb /rocksdb /rocksdb

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY *.go .
COPY api/ api/
COPY cmd/ cmd/
COPY controllers/ controllers/
COPY internal/ internal/

ARG VERSION

RUN export  CGO_ENABLED=1 \
            LDFLAGS='-linkmode external -extldflags "-static"' \
            CGO_CFLAGS="-I/rocksdb/include" \
            CGO_LDFLAGS="-L/rocksdb -L/usr/lib -L/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd";\
    go build -tags 'rocksdb pebbledb' -ldflags "-X github.com/strangelove-ventures/cosmos-operator/internal/version.version=$VERSION $LDFLAGS" -a -o manager .

# Build final image from scratch
FROM scratch

LABEL org.opencontainers.image.source=https://github.com/strangelove-ventures/cosmos-operator

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
