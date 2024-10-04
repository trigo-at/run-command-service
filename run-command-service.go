package main

import (
	"encoding/json"
	"errors"
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
	Command         string `yaml:"command"`
	RunInBackground bool   `yaml:"runInBackground"`
	RunOnce         bool   `yaml:"runOnce"`
}

var (
	config        Config
	executeSecret string
	shellPath     string
	listenPort    string
	mu            sync.Mutex
	isRunning     bool
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

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	log.Println("Starting Run Command Service")

	// Load configuration from file
	configPath := os.Getenv("CONFIG_FILE_PATH")
	if configPath == "" {
		// Set default config path to "config.yaml" in the same directory as the executable
		ex, err := os.Executable()
		if err != nil {
			return fmt.Errorf("error getting executable path: %v", err)
		}
		configPath = filepath.Join(filepath.Dir(ex), "config.yaml")
		log.Printf("CONFIG_FILE_PATH not set, using default: %s", configPath)
	}

	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		return fmt.Errorf("error parsing config file: %v", err)
	}

	if config.RunOnce && config.RunInBackground {
		return errors.New("runOnce and runInBackground cannot both be set to true")
	}

	// Get execute secret from environment variable
	executeSecret = os.Getenv("EXECUTE_SECRET")
	if executeSecret == "" {
		return fmt.Errorf("EXECUTE_SECRET environment variable is not set")
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
	log.Println("Command that will be executed:")
	log.Println("----------------------------------------")
	log.Println(expandedCommand)
	log.Println("----------------------------------------")

	// If RunOnce is true, execute the command and exit
	if config.RunOnce {
		return executeCommand(expandedCommand)
	}

	// Set up HTTP server
	http.HandleFunc("/ready", readyHandler)
	http.HandleFunc("/execute", executeHandler)

	log.Printf("Run Command Service starting on :%s", listenPort)
	return http.ListenAndServe(":"+listenPort, nil)
}

func executeCommand(command string) error {
	cmd := exec.Command(shellPath, "-c", command)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	return cmd.Wait()
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

	// Check if a background process is already running
	if isRunning && config.RunInBackground {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"status": "job still running in background"})
		return
	}

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

	// If running in background, return immediately
	if config.RunInBackground {
		isRunning = true
		go func() {
			io.Copy(os.Stdout, stdout)
			io.Copy(os.Stderr, stderr)
			cmd.Wait()
			mu.Lock()
			isRunning = false
			mu.Unlock()
		}()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "Process spawned successfully"})
		return
	}

	// For foreground execution, wait for the command to finish
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	err = cmd.Wait()

	// Prepare the response
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1 // Generic error code if we can't determine the actual exit code
		}
	}

	// Set the appropriate status code based on the exit code
	if exitCode != 0 {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	// Send JSON response with the exit code
	json.NewEncoder(w).Encode(map[string]int{"exit_code": exitCode})
}
