# Makefile for Run Command Service

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=run-command-service
BINARY_UNIX=$(BINARY_NAME)_unix

# Main package path
MAIN_PACKAGE=.

# Docker parameters
DOCKER=docker
DOCKER_IMAGE=run-command-service

all: test build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

run:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)
	./$(BINARY_NAME)

deps:
	$(GOGET) gopkg.in/yaml.v2

# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v $(MAIN_PACKAGE)

docker-build:
	$(DOCKER) build -t $(DOCKER_IMAGE) .

docker-run:
	$(DOCKER) run -p 8080:8080 -e RCS_EXECUTE_SECRET=your_secret_here $(DOCKER_IMAGE)

.PHONY: all build test clean run deps build-linux docker-build docker-run
