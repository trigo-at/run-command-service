# Run Command Service

Run Command Service is a minimal HTTP server written in Go, designed to run as a sidecar container and execute predefined commands when requested.

## Features

- Executes a predefined shell command via HTTP request
- Configurable through environment variables and a YAML configuration file
- Secure execution with secret key authentication
- Supports custom shell paths
- Provides a ready check endpoint

## Configuration

### Environment Variables

- `CONFIG_FILE_PATH`: Path to the YAML configuration file (default: `./config.yaml`)
- `EXECUTE_SECRET`: Secret key for authentication (required)
- `SHELL_PATH`: Path to the shell used for executing commands (default: `/bin/sh`)
- `LISTEN_PORT`: Port on which the service listens (default: `8080`)

### Configuration File (YAML)

The configuration file should contain a `command` key with the shell command to be executed.

Example configuration file (`config.yaml`):




