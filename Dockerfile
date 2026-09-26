# Stage 1: Build the statically linked Go binary with compiler cache mounts
FROM golang:1.22-alpine AS builder
WORKDIR /app

# Ensure we have CA certificates available for copy if needed
RUN apk --no-cache add ca-certificates

# Copy dependency manifests
COPY go.mod ./

# Download dependencies utilizing Docker BuildKit caching
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy source code
COPY . .

# Build statically linked binary with compiler and module cache mounts
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o xrpay ./cmd/xrpay

# Stage 2: Final micro-image with zero OS shell utilities
# Runs natively as nonroot (UID 65532) with pre-installed CA certificates
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=builder /app/xrpay /xrpay

EXPOSE 8080

# Execute statically compiled binary directly
ENTRYPOINT ["/xrpay"]
