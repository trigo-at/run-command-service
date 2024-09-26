# Start from the official Golang image
FROM golang:1.20-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o run-command-service .

# Start a new stage from scratch
FROM alpine:latest  

# Install bash
RUN apk add --no-cache bash

# Set the working directory
WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/run-command-service .

# Copy the config file
COPY config.yaml ./config.yaml

# Expose port 8080 to the outside world (default, can be overridden)
EXPOSE 8080

# Command to run the executable
CMD ["./run-command-service"]

# Set environment variables
ENV CONFIG_FILE_PATH=config.yaml
ENV SHELL_PATH=/bin/bash
ENV LISTEN_PORT=8080
