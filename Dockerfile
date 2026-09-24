# ── Stage 1: Build Go Binary ─────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git & SSL certs
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build standalone Linux binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server/main.go

# ── Stage 2: Minimal Production Image ─────────────────────────────
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy compiled binary from builder stage
COPY --from=builder /app/server /app/server

# Copy public assets (docs.html, static files) into production image
COPY --from=builder /app/public /app/public

# Expose Render default port
EXPOSE 8080

CMD ["/app/server"]
