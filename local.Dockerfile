FROM golang:1.20-alpine AS builder

RUN apk add --update --no-cache gcc libc-dev

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
COPY controllers/ controllers/
COPY internal/ internal/

ARG VERSION

RUN export CGO_ENABLED=1 LDFLAGS='-linkmode external -extldflags "-static"'; \
    go build -ldflags "-X github.com/strangelove-ventures/cosmos-operator/internal/version.version=$VERSION $LDFLAGS" -a -o manager .

# Build final image from scratch
FROM scratch

LABEL org.opencontainers.image.source=https://github.com/strangelove-ventures/cosmos-operator

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
