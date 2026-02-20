FROM golang:1.24.3-alpine AS builder

WORKDIR /app

# Disable CGO for static binary
ENV CGO_ENABLED=0 GOOS=linux

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the binary
RUN go build -o telegram-bot-nats .

# Create a clean, minimal runtime image
FROM alpine:latest

WORKDIR /app

# Install certificates for Telegram API HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from builder
COPY --from=builder /app/telegram-bot-nats .

EXPOSE 8080

CMD ["./telegram-bot-nats"]
