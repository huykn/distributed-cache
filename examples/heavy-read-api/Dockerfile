ARG BUILD_TARGET=writer

# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy root go mod files (for the distributed-cache library)
COPY go.mod go.sum ./
COPY *.go ./
COPY cache/ ./cache/
COPY storage/ ./storage/
COPY sync/ ./sync/
COPY types/ ./types/

# Copy example go mod files and source
COPY examples/heavy-read-api/go.mod examples/heavy-read-api/go.sum ./examples/heavy-read-api/
COPY examples/heavy-read-api/ ./examples/heavy-read-api/

# Download dependencies from the example directory
WORKDIR /build/examples/heavy-read-api
RUN go mod download

# Build the service binary
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
ARG BUILD_TARGET
RUN go build -ldflags="-s -w" -o /build/$BUILD_TARGET ./$BUILD_TARGET

# Runtime stage
FROM alpine:latest

ARG BUILD_TARGET=writer

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder - rename to 'service' for consistency
COPY --from=builder /build/$BUILD_TARGET ./service

EXPOSE 8080

CMD ["./service"]
