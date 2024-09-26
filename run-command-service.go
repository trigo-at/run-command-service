package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v2"
)

// Config struct to hold the command configuration
type Config struct {
	Command string `yaml:"command"`
}

var (
	config        Config
	executeSecret string
	shellPath     string
	listenPort    string
	mu            sync.Mutex
)

func main() {
	// Define help flag
	help := flag.Bool("help", false, "Print help information")
	flag.Parse()

	// If help flag is set, print help information and exit
	if *help {
		printHelp()
		os.Exit(0)
	}

	log.Println("Starting Run Command Service")

	// Load configuration from file
	configPath := os.Getenv("CONFIG_FILE_PATH")
	if configPath == "" {
		// Set default config path to "config.yaml" in the same directory as the executable
		ex, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		configPath = filepath.Join(filepath.Dir(ex), "config.yaml")
		log.Printf("CONFIG_FILE_PATH not set, using default: %s", configPath)
	}

	configFile, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}

	// Get execute secret from environment variable
	executeSecret = os.Getenv("EXECUTE_SECRET")
	if executeSecret == "" {
		log.Fatal("EXECUTE_SECRET environment variable is not set")
	}

	// Get shell path from environment variable or use default
	shellPath = os.Getenv("SHELL_PATH")
	if shellPath == "" {
		shellPath = "/bin/sh"
		log.Println("SHELL_PATH not set, defaulting to /bin/sh")
	}

	// Get listen port from environment variable or use default
	listenPort = os.Getenv("LISTEN_PORT")
	if listenPort == "" {
		listenPort = "8080"
		log.Println("LISTEN_PORT not set, defaulting to 8080")
	}

	// Print the command that will be executed
	expandedCommand := os.ExpandEnv(config.Command)
	log.Println("Command that will be executed on /execute:")
	log.Println("----------------------------------------")
	log.Println(expandedCommand)
	log.Println("----------------------------------------")

	// Set up HTTP server
	http.HandleFunc("/ready", readyHandler)
	http.HandleFunc("/execute", executeHandler)

	log.Printf("Run Command Service starting on :%s", listenPort)
	log.Fatal(http.ListenAndServe(":"+listenPort, nil))
}

// printHelp prints documentation about environment variables and config files
func printHelp() {
	helpText := `
Run Command Service

This service provides an HTTP API to execute predefined shell commands.

Environment Variables:
  CONFIG_FILE_PATH  : Path to the YAML configuration file (default: ./config.yaml)
  EXECUTE_SECRET    : Secret key for authentication (required)
  SHELL_PATH        : Path to the shell used for executing commands (default: /bin/sh)
  LISTEN_PORT       : Port on which the service listens (default: 8080)

Configuration File (YAML):
  The configuration file should contain a 'command' key with the shell command to be executed.

Example config.yaml:
  command: |
    echo "Hello from Run Command Service!"
    echo "Current date: $(date)"

Usage:
  run-command-service [flags]

Flags:
  --help    Print this help information

Endpoints:
  GET  /ready   : Returns 200 OK if the service is running
  POST /execute : Executes the configured command and returns its exit code
                  (requires 'x-secret' header for authentication)

For more information, please refer to the README.md file.
`
	fmt.Println(helpText)
}

// readyHandler handles the GET /ready endpoint
func readyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Run Command Service is running")
}

// executeHandler handles the POST /execute endpoint
func executeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for x-secret header
	secret := r.Header.Get("x-secret")
	if secret != executeSecret {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// Expand environment variables in the command
	expandedCommand := os.ExpandEnv(config.Command)

	// Execute the command using the specified shell
	cmd := exec.Command(shellPath, "-c", expandedCommand)

	// Set up pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating stdout pipe: %v", err), http.StatusInternalServerError)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating stderr pipe: %v", err), http.StatusInternalServerError)
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Error starting command: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy stdout and stderr to the server's stdout/stderr
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	// Wait for the command to finish
	err = cmd.Wait()

	// Prepare the response
	var exitCode int
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1 // Generic error code if we can't determine the actual exit code
		}
	}

	// Send JSON response with just the exit code
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"exit_code": exitCode})
}
