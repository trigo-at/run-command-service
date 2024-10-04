# Stage 1: Build and test
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

# Set the working directory
WORKDIR /app

# Install necessary build tools
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY *.go ./

# Run unit tests
RUN go test -v ./...

# Build arguments for version information
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=unknown

# Build the application
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "-X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME -X main.GitCommit=$GIT_COMMIT" -o run-command-service .

# Stage 2: Create the final lightweight image
FROM --platform=$TARGETPLATFORM alpine:latest

# Install bash
RUN apk add --no-cache bash

# Set the working directory
WORKDIR /app/

# Copy only the built binary from the previous stage
COPY --from=builder /app/run-command-service /usr/local/bin/run-command-service

# Copy the config file
COPY config.yaml /app/config.yaml

# Expose port 8080 to the outside world (default, can be overridden)
EXPOSE 8080

# Command to run the executable
CMD ["/usr/local/bin/run-command-service"]

# Set environment variables
ENV RCS_CONFIG_FILE_PATH=/app/config.yaml
ENV RCS_SHELL_PATH=/bin/bash
ENV RCS_RCS_RCS_LISTEN_PORT=8080