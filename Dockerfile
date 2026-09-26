# Stage 1: Build the statically linked Go binary
FROM golang:alpine AS builder
WORKDIR /app

# Copy dependency manifest
COPY go.mod ./

# Copy source code
COPY . .

# Build statically linked binary with stripped debug symbols
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o xrpay ./cmd/xrpay

# Stage 2: Final micro-image with zero OS shell utilities
# Runs natively as nonroot (UID 65532) with pre-installed CA certificates
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=builder /app/xrpay /xrpay

EXPOSE 8080

# Execute statically compiled binary directly
ENTRYPOINT ["/xrpay"]
