# Use RocksDB base image
FROM ghcr.io/strangelove-ventures/rocksdb:v7.10.2 AS rocksdb

# Use Alpine-based Go image for lightweight build
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

# Install dependencies
RUN apk add --update --no-cache \
    gcc \
    musl-dev \
    libc-dev \
    g++ \
    make \
    git \
    bash \
    linux-headers \
    perl \
    snappy-dev \
    zlib-dev \
    bzip2-dev \
    lz4-dev \
    zstd-dev \
    binutils \
    wget

# Set build arguments
ARG TARGETARCH
ARG BUILDARCH

# Install cross-compilers if needed
RUN if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
        wget -q -O - https://musl.cc/aarch64-linux-musl-cross.tgz | tar -xz --strip-components 1 -C /usr; \
        which aarch64-linux-musl-gcc || echo "Cross compiler missing!"; \
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
        wget -q -O - https://musl.cc/x86_64-linux-musl-cross.tgz | tar -xz --strip-components 1 -C /usr; \
        which x86_64-linux-musl-gcc || echo "Cross compiler missing!"; \
    fi

# Install static versions of required libraries
RUN apk add --update --no-cache \
    snappy-static \
    zlib-static \
    bzip2-static \
    lz4-static \
    zstd-static

# Copy RocksDB headers and static libraries
COPY --from=rocksdb /rocksdb /rocksdb

# Set working directory
WORKDIR /workspace

# Copy Go module files
COPY go.mod go.sum ./

# Download dependencies to cache before copying full source
RUN go mod download

# Copy all Go source files
COPY . .

# Define build arguments
ARG VERSION

# Build binary with improved cross-compilation
RUN set -eux; \
    echo "Building for TARGETARCH=${TARGETARCH}, BUILDARCH=${BUILDARCH}"; \
    if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
        export CC=/usr/bin/aarch64-linux-musl-gcc CXX=/usr/bin/aarch64-linux-musl-g++; \
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
        export CC=/usr/bin/x86_64-linux-musl-gcc CXX=/usr/bin/x86_64-linux-musl-g++; \
    fi; \
    export GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=1; \
    export LDFLAGS="-s -w -extldflags '-static -lpthread -ldl'"; \
    export CGO_CFLAGS="-I/rocksdb/include -I/usr/include"; \
    export CGO_LDFLAGS="-L/rocksdb -L/usr/lib -L/lib -lrocksdb -lzstd -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -latomic"; \
    go build -v -x -tags 'rocksdb pebbledb' -ldflags "-X github.com/strangelove-ventures/cosmos-operator/internal/version.version=$VERSION $LDFLAGS" -o manager

# Verify built binary
RUN file manager && ldd manager || true

# Build minimal final container
FROM scratch

LABEL org.opencontainers.image.source=https://github.com/strangelove-ventures/cosmos-operator

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
