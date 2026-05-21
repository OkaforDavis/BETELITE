FROM golang:alpine

WORKDIR /app

# Install certificates for secure LiveKit connection
RUN apk add --no-cache ca-certificates

# Copy go files and download modules
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

# Copy the actual code and the frontend mobile files
COPY backend/ ./backend/
COPY mobile/ ./mobile/

# Build the Go app
WORKDIR /app/backend
RUN go build -o server .

EXPOSE 3000

CMD ["./server"]
