package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
)

func TestReadyHandler(t *testing.T) {
	// Create a request to pass to our handler
	req, err := http.NewRequest("GET", "/ready", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(readyHandler)

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the response body
	expected := map[string]string{"status": "ok"}
	var got map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &got)
	if err != nil {
		t.Fatal(err)
	}
	if got["status"] != expected["status"] {
		t.Errorf("handler returned unexpected body: got %v want %v", got, expected)
	}
}

func TestExecuteHandler(t *testing.T) {
	// Set up test configuration
	config = Config{Command: "echo 'test'"}
	executeSecret = "test-secret"
	secretHeader = "x-secret"
	shellPath = "/bin/sh"

	tests := []struct {
		name           string
		method         string
		secret         string
		expectedStatus int
		expectedCode   int
	}{
		{"Valid request", "POST", "test-secret", http.StatusOK, 0},
		{"Invalid method", "GET", "test-secret", http.StatusMethodNotAllowed, 0},
		{"Invalid secret", "POST", "wrong-secret", http.StatusUnauthorized, 0},
		{"Failed command", "POST", "test-secret", http.StatusInternalServerError, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For the "Failed command" test, temporarily change the command
			if tt.name == "Failed command" {
				oldConfig := config
				config = Config{Command: "exit 1"}
				defer func() { config = oldConfig }()
			}

			req, err := http.NewRequest(tt.method, "/execute", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("x-secret", tt.secret)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(executeHandler)

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedStatus)
			}

			if tt.expectedStatus == http.StatusOK || tt.expectedStatus == http.StatusInternalServerError {
				var response map[string]int
				err = json.Unmarshal(rr.Body.Bytes(), &response)
				if err != nil {
					t.Fatal(err)
				}
				if response["exit_code"] != tt.expectedCode {
					t.Errorf("handler returned unexpected exit code: got %v want %v", response["exit_code"], tt.expectedCode)
				}
			}
		})
	}
}

func TestExecuteHandlerWithRequestData(t *testing.T) {
	// Create a temporary file to capture command output
	tmpFile, err := os.CreateTemp("", "request-data-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// Set up test configuration with a simple command
	config = Config{Command: "printenv REQUEST_DATA > " + tmpFile.Name()}
	executeSecret = "test-secret"
	secretHeader = "x-secret"
	shellPath = "/bin/sh"

	// Test data
	testData := `{"test":"data","number":123}`

	// Create a request with the test data in the body
	req, err := http.NewRequest("POST", "/execute", bytes.NewBufferString(testData))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-secret", "test-secret")

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(executeHandler)

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Log the response for debugging
	log.Printf("Response status: %d", rr.Code)
	log.Printf("Response body: %s", rr.Body.String())

	// Wait for the command to complete
	time.Sleep(500 * time.Millisecond)

	// Read the temp file to verify the REQUEST_DATA was correctly passed
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Log the file content
	log.Printf("File content (length: %d): %s", len(content), string(content))

	// Trim any whitespace or newlines
	actualData := strings.TrimSpace(string(content))

	if actualData != testData {
		t.Errorf("REQUEST_DATA not correctly passed to command: got %q want %q", actualData, testData)
	}
}

func TestExecuteHandlerWithBackgroundOption(t *testing.T) {
	// Set up test configuration
	config = Config{
		Command:         "sleep 2 && echo 'test'",
		RunInBackground: true,
	}
	executeSecret = "test-secret"
	secretHeader = "x-secret"
	shellPath = "/bin/sh"

	req, err := http.NewRequest("POST", "/execute", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-secret", "test-secret")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(executeHandler)

	start := time.Now()
	handler.ServeHTTP(rr, req)
	duration := time.Since(start)

	// Check if the response was quick (less than the sleep duration)
	if duration >= 2*time.Second {
		t.Errorf("handler took too long to respond: %v", duration)
	}

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the response body
	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatal(err)
	}
	expected := "Process spawned successfully"
	if response["status"] != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", response["status"], expected)
	}

	// Wait a bit and check if the output was captured
	time.Sleep(3 * time.Second)
	// Note: In a real test environment, you might want to capture os.Stdout
	// and check its content instead of this comment.
	// For simplicity, we're just waiting here.
}

func TestExecuteCommandWithRequestData(t *testing.T) {
	// Create a temporary file to capture command output
	tmpFile, err := os.CreateTemp("", "execute-command-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// Set shell path for the test
	shellPath = "/bin/sh"

	// Test data
	testData := []byte(`{"test":"value","array":[1,2,3]}`)

	// Command that writes the REQUEST_DATA to the temp file
	command := "echo $REQUEST_DATA > " + tmpFile.Name()

	// Execute the command with the test data
	err = executeCommand(command, testData)
	if err != nil {
		t.Fatalf("executeCommand failed: %v", err)
	}

	// Read the temp file to verify the REQUEST_DATA was correctly passed
	time.Sleep(100 * time.Millisecond) // Small delay to ensure file is written
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Trim any whitespace or newlines
	actualData := strings.TrimSpace(string(content))
	expectedData := string(testData)
	if actualData != expectedData {
		t.Errorf("REQUEST_DATA not correctly passed to command: got %v want %v", actualData, expectedData)
	}
}

func TestRunOnceOption(t *testing.T) {
	tests := []struct {
		name            string
		command         string
		runOnce         bool
		runInBackground bool
		expectedErr     bool
		expectedErrMsg  string
	}{
		{"Successful command", "echo 'test'", true, false, false, ""},
		{"Failed command", "exit 1", true, false, true, ""},
		{"Mutually exclusive options", "echo 'test'", true, true, true, "runOnce and runInBackground cannot both be set to true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for the config file
			tmpDir, err := os.MkdirTemp("", "test-config")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create a temporary config file
			configPath := filepath.Join(tmpDir, "config.yaml")
			configContent := Config{
				Command:         tt.command,
				RunOnce:         tt.runOnce,
				RunInBackground: tt.runInBackground,
			}
			configData, err := yaml.Marshal(configContent)
			if err != nil {
				t.Fatalf("Failed to marshal config: %v", err)
			}
			err = os.WriteFile(configPath, configData, 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			// Set the RCS_CONFIG_FILE_PATH environment variable
			os.Setenv("RCS_CONFIG_FILE_PATH", configPath)
			defer os.Unsetenv("RCS_CONFIG_FILE_PATH")

			// Set other required environment variables
			os.Setenv("RCS_EXECUTE_SECRET", "test-secret")
			defer os.Unsetenv("RCS_EXECUTE_SECRET")

			shellPath = "/bin/sh"

			err = run()

			if (err != nil) != tt.expectedErr {
				t.Errorf("run() error = %v, expectedErr %v", err, tt.expectedErr)
			}

			if tt.expectedErrMsg != "" && (err == nil || err.Error() != tt.expectedErrMsg) {
				t.Errorf("run() error message = %v, expectedErrMsg %v", err, tt.expectedErrMsg)
			}
		})
	}
}

func TestExecuteHandlerWithCustomHeaderName(t *testing.T) {
	// Save original values to restore later
	originalSecretHeader := secretHeader
	originalExecuteSecret := executeSecret

	// Set up test configuration
	config = Config{Command: "echo 'test'"}
	executeSecret = "custom-secret-value"
	secretHeader = "custom-auth-header" // Set custom header name
	shellPath = "/bin/sh"

	// Create a request with the custom header
	req, err := http.NewRequest("POST", "/execute", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("custom-auth-header", "custom-secret-value") // Use custom header name

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(executeHandler)

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the response body
	var response map[string]int
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatal(err)
	}
	if response["exit_code"] != 0 {
		t.Errorf("handler returned unexpected exit code: got %v want %v", response["exit_code"], 0)
	}

	// Test with incorrect header name (should fail)
	req, err = http.NewRequest("POST", "/execute", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-secret", "custom-secret-value") // Use default header name instead of custom

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should return unauthorized
	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler should return unauthorized with wrong header name: got %v want %v",
			status, http.StatusUnauthorized)
	}

	// Restore original values
	secretHeader = originalSecretHeader
	executeSecret = originalExecuteSecret
}

func TestMain(m *testing.M) {
	// Set up test environment
	os.Setenv("RCS_EXECUTE_SECRET", "test-secret")
	os.Setenv("RCS_SHELL_PATH", "/bin/sh")
	os.Setenv("RCS_LISTEN_PORT", "8080")
	// Run tests
	code := m.Run()

	// Clean up
	os.Unsetenv("RCS_EXECUTE_SECRET")
	os.Unsetenv("RCS_SHELL_PATH")
	os.Unsetenv("RCS_LISTEN_PORT")

	os.Exit(code)
}
