FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o betelite .

FROM alpine:3.22
RUN apk --no-cache add ca-certificates tzdata && adduser -D -H app
WORKDIR /app
COPY --from=builder /app/betelite .
COPY --from=builder /app/static ./static
USER app
EXPOSE 3000
CMD ["./betelite"]
