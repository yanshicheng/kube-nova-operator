# syntax=docker/dockerfile:1.4
# Build the manager binary
FROM registry.cn-hangzhou.aliyuncs.com/kube-nova/golang:1.25.5-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH} \
    GOPROXY=https://goproxy.cn,direct

# Copy the Go Modules manifests
COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod,id=go-mod \
    go mod download

# Copy the Go source
COPY . .

# Build with cache mount
RUN --mount=type=cache,target=/go/pkg/mod,id=go-mod \
    --mount=type=cache,target=/root/.cache/go-build,id=go-build \
    go build -ldflags="-s -w" -o manager cmd/main.go

# Use distroless as minimal base image
FROM registry.cn-hangzhou.aliyuncs.com/kube-nova/static:nonroot 
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]