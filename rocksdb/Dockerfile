FROM --platform=$BUILDPLATFORM golang:1.20-alpine AS builder

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

RUN if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
        wget -c https://musl.cc/aarch64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr; \
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
        wget -c https://musl.cc/x86_64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr; \
    fi

ARG ROCKSDB_VERSION=v7.10.2

# Install RocksDB
WORKDIR /
RUN git clone -b ${ROCKSDB_VERSION} --single-branch https://github.com/facebook/rocksdb.git

WORKDIR /rocksdb

RUN set -eux;\
    if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
        echo aarch64 > /etc/apk/arch;\
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
        echo x86_64 > /etc/apk/arch;\
    fi;\
    apk add --update --no-cache\
    snappy-static\
    zlib-static\
    bzip2-static\
    lz4-static\
    zstd-static\
    --allow-untrusted

RUN if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
        export CC=aarch64-linux-musl-gcc CXX=aarch64-linux-musl-g++;\
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
        export CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++; \
    fi; \
    PORTABLE=1 make -j$(nproc) static_lib
