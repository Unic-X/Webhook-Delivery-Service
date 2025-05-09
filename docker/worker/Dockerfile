FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy all files
COPY . .

# Build the worker binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/worker ./cmd/worker

# Create a minimal image
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/worker .
COPY --from=builder /app/.env .

# Run the worker binary
CMD ["./worker"]
