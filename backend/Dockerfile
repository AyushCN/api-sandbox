# Stage 1: Build the Go binary
FROM golang:1.22-alpine AS builder

# Install required build tools
RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o api-sandbox-backend .

# Stage 2: Create a minimal runner image
FROM alpine:3.19

# Install required runtime dependencies (Docker client needs to be available if executing raw docker commands, though we use the Go SDK. However, Nixpacks and git are required for buildpack auto-detection and clone)
RUN apk add --no-cache ca-certificates git docker-cli curl bash

# Create an app directory and set it as the working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/api-sandbox-backend /usr/local/bin/

# We run as root here because the container requires direct access to /var/run/docker.sock
# which is owned by root on most host machines. 
# WARNING: This grants the container host-level root equivalent access.

# Create workspaces directory
RUN mkdir -p /app/workspaces

EXPOSE 8080

CMD ["api-sandbox-backend"]
