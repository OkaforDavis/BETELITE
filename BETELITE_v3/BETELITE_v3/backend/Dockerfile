FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o betelite .

# Run stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/betelite .

# Copy env file if available
# Note: In production, environment variables should be injected via docker run or compose
COPY .env* ./

EXPOSE 3000

CMD ["./betelite"]
