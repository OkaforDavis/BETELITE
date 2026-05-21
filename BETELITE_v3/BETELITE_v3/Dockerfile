FROM golang:alpine

WORKDIR /app

# Install certificates for secure LiveKit connection
RUN apk add --no-cache ca-certificates

# Copy go files and download modules
COPY go_streaming/go.mod go_streaming/go.sum ./go_streaming/
RUN cd go_streaming && go mod download

# Copy the actual code and the frontend mobile files
COPY go_streaming/ ./go_streaming/
COPY mobile/ ./mobile/

# Build the Go app
WORKDIR /app/go_streaming
RUN go build -o server .

EXPOSE 3000

CMD ["./server"]
