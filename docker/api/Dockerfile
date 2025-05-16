FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy all files
COPY . .

# Build the API binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api ./cmd/api

# Create a minimal image
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/api .
COPY --from=builder /app/migrations ./migrations

# Run the API binary
CMD ["./api"]
