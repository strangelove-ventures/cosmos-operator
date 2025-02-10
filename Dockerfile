# Use RocksDB base image
FROM ghcr.io/strangelove-ventures/rocksdb:v7.10.2 AS rocksdb

# Use Alpine-based Go image for lightweight build
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

# Install required dependencies
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
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
        wget -q -O - https://musl.cc/x86_64-linux-musl-cross.tgz | tar -xz --strip-components 1 -C /usr; \
    fi

# Verify correct compiler installation
RUN if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
        ls /usr/bin | grep aarch64-linux-musl-gcc; \
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
        ls /usr/bin | grep x86_64-linux-musl-gcc; \
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

# Build the binary with proper static linking
RUN set -eux; \
    echo "Building for TARGETARCH=${TARGETARCH}, BUILDARCH=${BUILDARCH}"; \
    if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
        export CC=aarch64-linux-musl-gcc CXX=aarch64-linux-musl-g++; \
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
        export CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++; \
    fi; \
    export GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=1; \
    export LDFLAGS="-linkmode external -extldflags '-static'"; \
    export CGO_CFLAGS="-I/rocksdb/include"; \
    export CGO_LDFLAGS="-L/rocksdb -L/usr/lib -L/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd"; \
    go build -v -x -tags 'rocksdb pebbledb' -ldflags "-X github.com/strangelove-ventures/cosmos-operator/internal/version.version=$VERSION $LDFLAGS" -o manager

# Verify the built binary
RUN file manager && ldd manager || true

# Build final minimal container
FROM scratch

LABEL org.opencontainers.image.source=https://github.com/strangelove-ventures/cosmos-operator

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
