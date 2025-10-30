FROM golang:1.23-alpine

WORKDIR /app

# Install build dependencies for cgo and sqlite3
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Enable CGO
ENV CGO_ENABLED=1

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o server .

# Expose port
EXPOSE 4000

# Start the application
CMD ["./server"]
