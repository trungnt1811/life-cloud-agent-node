# =====================
# Build stage
# =====================
FROM golang:1.24.7-alpine AS builder
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go.mod and go.sum and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code
COPY . .

# Build the main application binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/main.go

# Build the migration tool binary
RUN CGO_ENABLED=0 GOOS=linux go build -o migrate ./cmd/migration/main.go

# =====================
# Runtime stage
# =====================
FROM alpine:3.19
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/main .
COPY --from=builder /app/migrate .

# Copy migration scripts
COPY --from=builder /app/internal/adapters/postgres/scripts /app/internal/adapters/postgres/scripts

# Create a non-root user and switch to it
RUN adduser -D appuser
USER appuser

EXPOSE 8080

# Default command to run the main application
CMD ["./main"]
