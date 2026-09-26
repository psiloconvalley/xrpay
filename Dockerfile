# Stage 1: Build the vanilla Go binary
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o xrpay ./cmd/xrpay

# Stage 2: Final minimal runtime image
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/xrpay .

# Run as non-privileged system user for defense-in-depth
RUN adduser -D -u 10001 appuser
USER appuser

EXPOSE 8080
ENTRYPOINT ["./xrpay"]
