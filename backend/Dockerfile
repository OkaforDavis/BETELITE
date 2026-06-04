FROM golang:1.24-alpine AS builder

WORKDIR /app

# Allow Go to automatically download newer toolchain if dependencies require it
ENV GOTOOLCHAIN=auto

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

EXPOSE 3000

CMD ["./betelite"]
