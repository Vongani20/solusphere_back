# Build stage - using Go 1.24 or higher
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/server/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/main .

# Create migrations directory and copy migrations if they exist
RUN mkdir -p /root/migrations

# Copy migrations from builder (if any)
COPY --from=builder /app/migrations /root/migrations 

# If no migrations exist, create a default one
RUN if [ ! "$(ls -A /root/migrations)" ]; then \
    echo "Creating default migration file..."; \
    echo '-- Initial schema migration' > /root/migrations/000001_initial_schema.up.sql; \
    echo 'CREATE TABLE IF NOT EXISTS users (id INT AUTO_INCREMENT PRIMARY KEY, email VARCHAR(255) NOT NULL UNIQUE, name VARCHAR(255), password VARCHAR(255), created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, deleted_at TIMESTAMP NULL);' >> /root/migrations/000001_initial_schema.up.sql; \
    echo 'DROP TABLE IF EXISTS users;' > /root/migrations/000001_initial_schema.down.sql; \
    fi

# Expose port
EXPOSE 2080

# Run the binary
CMD ["./main"]