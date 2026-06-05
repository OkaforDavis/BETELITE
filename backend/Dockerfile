FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and static frontend
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o betelite .

# Run stage - minimal alpine image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary and static frontend files
COPY --from=builder /app/betelite .
COPY --from=builder /app/static ./static

EXPOSE 3000

CMD ["./betelite"]
