# Stage 1: Build the Go application
FROM golang:1.24-alpine AS builder

# Set necessary environment variables
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Set the working directory inside the container
WORKDIR /app/src

# Copy go module files
COPY src/go.mod src/go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY src/ ./

# Build the application binary statically
# Assuming your main package is in the root or ./cmd/yoopass-api/
# Adjust the path './...' or specific package if needed
# The path is now relative to the WORKDIR (/app/src)
RUN go build -ldflags="-w -s" -o /src ./main.go

# Stage 2: Create the final lightweight image
FROM alpine:latest

# Copy the static binary from the builder stage
COPY --from=builder /src /src
# Copy configuration files (adjust path if your config is elsewhere)
COPY config /config

# Expose the port the application runs on (adjust if different from 8082)
EXPOSE 8082

# Set the entrypoint for the container
ENTRYPOINT ["/src"]