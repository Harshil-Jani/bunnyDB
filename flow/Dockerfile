# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files and download dependencies
COPY go.mod ./
RUN go mod download || true

# Copy source code and generate go.sum
COPY . .
RUN go mod tidy

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o bunny ./cmd/main.go

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy the binary from builder
COPY --from=builder /app/bunny .

# Create non-root user
RUN adduser -D -g '' bunny
USER bunny

ENTRYPOINT ["./bunny"]
