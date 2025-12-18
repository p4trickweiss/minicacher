FROM --platform=$BUILDPLATFORM golang:1.25.5 AS builder
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


# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/distributed-cache .
ENTRYPOINT ["/distributed-cache"]
