# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o kube-watcher ./cmd

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

# Run as non-root user
RUN adduser -D -u 1000 watcher

WORKDIR /home/watcher

# Copy the binary from builder
COPY --from=builder /app/kube-watcher .

# Change ownership to watcher user
RUN chown watcher:watcher /home/watcher/kube-watcher && \
    chmod +x /home/watcher/kube-watcher

USER watcher

ENTRYPOINT ["./kube-watcher"]
