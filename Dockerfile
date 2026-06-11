# syntax=docker/dockerfile:1
# Build stage
ARG GO_VERSION=1.26.3
FROM golang:${GO_VERSION}-alpine AS builder

# Install build dependencies for CGO (needed for SQLite)
RUN apk add --no-cache build-base

WORKDIR /app

# Copy source code
COPY . .

# Build the application.
# CGO_ENABLED=1 is required for the standard SQLite driver.
# BuildKit cache mounts persist the Go build cache and module downloads across
# image builds (incremental recompiles + no module re-download — keeper has no vendor dir).
# Dropped `-a -installsuffix cgo` — those forced a full rebuild and defeated the cache.
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 CGO_CFLAGS="-D_LARGEFILE64_SOURCE" GOOS=linux \
    go build -o keeper ./cmd/api/main.go

# Final stage
FROM alpine:3.22

RUN apk --no-cache add ca-certificates sqlite-libs

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/keeper .

# Expose port 8080
EXPOSE 8080

# Command to run the executable
CMD ["./keeper"]
