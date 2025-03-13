# Base image for building
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

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
    zstd-dev

ARG TARGETARCH
ARG BUILDARCH

# Install cross-compiler tools if needed
RUN if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
        wget -c https://musl.cc/aarch64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr; \
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
        wget -c https://musl.cc/x86_64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr; \
    fi

# Install static libraries
RUN apk add --update --no-cache\
    snappy-static\
    zlib-static\
    bzip2-static\
    lz4-static\
    zstd-static

# Build RocksDB from source for the target architecture
RUN git clone --branch v9.8.4 --depth 1 https://github.com/facebook/rocksdb.git /rocksdb-src && \
    cd /rocksdb-src && \
    if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
        PORTABLE=1 CC=aarch64-linux-musl-gcc CXX=aarch64-linux-musl-g++ TARGET_ARCHITECTURE=aarch64 make -j$(nproc) static_lib; \
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
        PORTABLE=1 CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++ make -j$(nproc) static_lib; \
    else \
        PORTABLE=1 make -j$(nproc) static_lib; \
    fi && \
    mkdir -p /rocksdb/include && \
    cp /rocksdb-src/librocksdb.a /rocksdb/ && \
    cp -r /rocksdb-src/include/* /rocksdb/include/ && \
    rm -rf /rocksdb-src

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# Cache deps
RUN go mod download

# Copy the go source
COPY *.go .
COPY api/ api/
COPY cmd/ cmd/
COPY controllers/ controllers/
COPY internal/ internal/

ARG VERSION

RUN set -eux;\
    if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then\
        export CC=aarch64-linux-musl-gcc CXX=aarch64-linux-musl-g++;\
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then\
        export CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++;\
    fi;\
    export  GOOS=linux \
            GOARCH=$TARGETARCH \
            CGO_ENABLED=1 \
            LDFLAGS='-linkmode external -extldflags "-static"' \
            CGO_CFLAGS="-I/rocksdb/include" \
            CGO_LDFLAGS="-L/rocksdb -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd";\
    go build -tags 'rocksdb pebbledb' -ldflags "-X github.com/strangelove-ventures/cosmos-operator/internal/version.version=$VERSION $LDFLAGS" -a -o manager .

# Build final image from scratch
FROM scratch

LABEL org.opencontainers.image.source=https://github.com/strangelove-ventures/cosmos-operator

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
