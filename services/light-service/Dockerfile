# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# CGO_ENABLED=0 for static binary (works with scratch/alpine)
# -ldflags="-w -s" strips debug info (smaller binary)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/server \
    ./cmd/server

# Runtime stage
FROM alpine:latest

# Install ca-certificates for TLS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/server .

# Expose gRPC port
EXPOSE 50051

# Run the server
CMD ["./server"]