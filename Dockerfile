# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY main.go ./

# Build the binary (no CGO needed)
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o tg-hacker-news main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 appgroup && \
    adduser -D -u 1001 -G appgroup appuser

# Create data directory
RUN mkdir -p /app/data && \
    chown -R appuser:appgroup /app

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/tg-hacker-news .

# Set ownership and permissions
RUN chown appuser:appgroup tg-hacker-news && \
    chmod +x tg-hacker-news

# Switch to non-root user
USER appuser

# Set environment variables
ENV DATA_PATH=/app/data/stories.json

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD [ -f /app/data/stories.json ] || exit 1

# Expose port (optional, for future web interface)
EXPOSE 8080

# Run the binary
CMD ["./tg-hacker-news"]