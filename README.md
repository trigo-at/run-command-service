# Run Command Service

Run Command Service is a lightweight, configurable HTTP service that executes predefined shell commands. It provides a secure way to trigger command execution via HTTP requests, making it useful for various automation and integration scenarios.

## Table of Contents

- [Run Command Service](#run-command-service)
  - [Table of Contents](#table-of-contents)
  - [Features](#features)
  - [Installation](#installation)
    - [Prerequisites](#prerequisites)
    - [Building from Source](#building-from-source)
  - [Configuration](#configuration)
    - [Environment Variables](#environment-variables)
    - [Configuration File](#configuration-file)
  - [Usage](#usage)
    - [Starting the Service](#starting-the-service)
    - [API Endpoints](#api-endpoints)
      - [GET /ready](#get-ready)
      - [POST /execute](#post-execute)
  - [Docker Support](#docker-support)
    - [Building the Docker Image](#building-the-docker-image)
    - [Pulling the Pre-built Image](#pulling-the-pre-built-image)
    - [Running the Container](#running-the-container)
  - [Security Considerations](#security-considerations)
  - [Development](#development)
    - [Running Tests](#running-tests)
    - [Makefile](#makefile)
  - [Troubleshooting](#troubleshooting)
  - [License](#license)
    - [MIT License Summary](#mit-license-summary)

## Features

- Execute predefined shell commands via HTTP requests
- Configurable through environment variables and a YAML configuration file
- Secure execution with secret-based authentication
- Docker support for easy deployment
- Customizable shell and listening port
- Returns command exit codes for easy integration
- Option to run commands in the background
- Run-once mode for single command execution

## Installation

### Prerequisites

- Go 1.20 or later
- Docker (optional, for containerized deployment)

### Building from Source

1. Clone the repository:
   ```
   git clone https://github.com/trigo-at/run-command-service.git
   cd run-command-service
   ```

2. Build the binary:
   ```
   go build -o run-command-service
   ```

## Configuration

### Environment Variables

The service can be configured using the following environment variables:

- `CONFIG_FILE_PATH`: Path to the YAML configuration file (default: `./config.yaml`)
- `EXECUTE_SECRET`: Secret key for authentication (required)
- `SHELL_PATH`: Path to the shell used for executing commands (default: `/bin/sh`)
- `LISTEN_PORT`: Port on which the service listens (default: `8080`)

### Configuration File

The service uses a YAML configuration file to define the command to be executed and its execution mode. By default, it looks for `config.yaml` in the same directory as the executable.

Example `config.yaml`:

```yaml
command: |
  echo "Hello from Run Command Service!"
  echo "Current date: $(date)"
  echo "Custom environment variable: $CUSTOM_VAR"
runInBackground: false
runOnce: false
```

- `command`: The shell command to be executed.
- `runInBackground`: If set to `true`, the command will be spawned as a background process.
- `runOnce`: If set to `true`, the service will execute the command once at startup and exit, using the command's exit code as its own.

Note: `runInBackground` and `runOnce` cannot both be set to `true` as they are mutually exclusive options.

## Usage

### Starting the Service

1. Set the required environment variables:
   ```
   export EXECUTE_SECRET=your_secret_here
   ```

2. Run the service:
   ```
   ./run-command-service
   ```

The service will start and display the configured command without executing it.

### API Endpoints

#### GET /ready

- **Description**: Checks if the service is running
- **Response**: 
  - Status Code: 200 OK
  - Content-Type: application/json
  - Body: JSON object indicating the service is running
    ```json
    {"status": "Run Command Service is running"}
    ```

#### POST /execute

- **Description**: Executes the configured command
- **Headers**:
  - `x-secret`: The secret key for authentication (must match `EXECUTE_SECRET`)
- **Response**:
  - For foreground execution (`runInBackground: false`):
    - Status Code: 
      - 200 OK if the command executed successfully (exit code 0)
      - 500 Internal Server Error if the command failed (non-zero exit code)
    - Body: JSON object containing the exit code
      ```json
      {"exit_code": 0}
      ```
  - For background execution (`runInBackground: true`):
    - Status Code: 
      - 200 OK if the process was successfully spawned
      - 409 Conflict if a background process is already running
    - Body: JSON object indicating the status
      ```json
      {"status": "Process spawned successfully"}
      ```
      or
      ```json
      {"status": "job still running in background"}
      ```

## Docker Support

You can run the Run Command Service using a pre-built Docker image from the GitHub Container Registry (ghcr.io) or build your own image. The service supports multiple architectures, including amd64 and arm64.

### Building the Docker Image

The Dockerfile uses a multi-stage build process that includes running unit tests and supports multi-architecture builds:

1. The first stage builds the application and runs unit tests.
2. The second stage creates a lean production image with only the necessary components.

To build the Docker image for your current architecture:

```bash
docker build -t run-command-service .
```

To build for multiple architectures using buildx:

```bash
docker buildx create --use
docker buildx build --platform linux/amd64,linux/arm64 -t run-command-service --push .
```

This process ensures that:
- Unit tests are run as part of the build process.
- The final image only contains the production binary, not the test code.
- Images are built for multiple architectures (amd64 and arm64).

### Pulling the Pre-built Image

To pull the latest version of the Docker image:

```bash
docker pull ghcr.io/trigo-at/run-command-service:latest
```

Docker will automatically pull the correct image for your architecture.

You can also pull a specific version or branch of the service by changing the tag:

```bash
docker pull ghcr.io/trigo-at/run-command-service:v1.0.0
# or
docker pull ghcr.io/trigo-at/run-command-service:main
```

### Running the Container

After pulling the image, you can run it with:

```bash
docker run -p 8080:8080 \
  -e EXECUTE_SECRET=your_secret_here \
  -e CONFIG_FILE_PATH=/app/config.yaml \
  -v /path/to/your/config.yaml:/app/config.yaml \
  ghcr.io/trigo-at/run-command-service:latest
```

Make sure to replace `/path/to/your/config.yaml` with the actual path to your configuration file on the host machine.

## Security Considerations

- Keep the `EXECUTE_SECRET` confidential and use a strong, unique value.
- Be cautious about the commands you configure, as they will be executed with the permissions of the user running the service.
- Consider running the service in a restricted environment or container for additional security.
- Use HTTPS in production to encrypt traffic between clients and the service.

## Development

### Running Tests

To run the test suite:

```
go test ./...
```

### Makefile

A Makefile is provided for common development tasks:

- `make build`: Build the binary
- `make test`: Run tests
- `make run`: Build and run the service
- `make docker-build`: Build the Docker image
- `make docker-run`: Run the service in a Docker container

## Troubleshooting

- If the service fails to start, check that all required environment variables are set correctly.
- Verify that the `config.yaml` file is in the correct location and properly formatted.
- Check the logs for any error messages or unexpected behavior.
- Ensure that the configured command in `config.yaml` is valid and can be executed by the specified shell.
- If both `runInBackground` and `runOnce` are set to `true` in the configuration, the service will return an error as these options are mutually exclusive.

For more information or to report issues, please visit the [GitHub repository](https://github.com/trigo-at/run-command-service).

## License

Run Command Service is open-source software licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.
### MIT License Summary

- You are free to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the software.
- You must include the copyright notice and the permission notice in all copies or substantial portions of the software.
- The software is provided "as is", without warranty of any kind.

For the full license text, please see the [LICENSE](LICENSE) file in the repository.

