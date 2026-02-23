# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

# Copy go mod files for both services (plant-service depends on light-service locally)
COPY services/light-service/go.mod services/light-service/go.sum ./services/light-service/
COPY services/plant-service/go.mod services/plant-service/go.sum ./services/plant-service/

WORKDIR /app/services/plant-service
RUN go mod download

WORKDIR /app
COPY services/light-service/ ./services/light-service/
COPY services/plant-service/ ./services/plant-service/

WORKDIR /app/services/plant-service

# CGO_ENABLED=0: no SQLite in plant-service — produces a fully static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/server \
    ./cmd/server

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/server .

EXPOSE 50052

CMD ["./server"]
