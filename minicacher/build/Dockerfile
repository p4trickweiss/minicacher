FROM --platform=$BUILDPLATFORM golang:1.26.2 AS builder
WORKDIR /workspace

# Copy the Go Modules manifests from parent directory
COPY ../go.mod go.mod
COPY ../go.sum go.sum

# Cache deps before building and copying source
RUN go mod download

# Copy the go source from parent directory
COPY ../cmd/main.go main.go
COPY ../internal/ internal/

# Build
ARG TARGETOS TARGETARCH
RUN GOOS="$TARGETOS" GOARCH="$TARGETARCH" CGO_ENABLED=0 go build -a -o distributed-cache main.go


FROM alpine:3.23
# Create non-root user
RUN addgroup -S app && adduser -S app -G app

# Writable data directory for Raft
RUN mkdir -p /data && chown -R app:app /data

WORKDIR /data

# Copy binary
COPY --from=builder /workspace/distributed-cache /usr/bin/distributed-cache

USER app

WORKDIR /data

ENTRYPOINT ["/usr/bin/distributed-cache"]
