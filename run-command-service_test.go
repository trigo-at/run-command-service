package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestExecuteHandlerWithBackgroundOption(t *testing.T) {
	// Set up test configuration
	config = Config{
		Command:         "sleep 2 && echo 'test'",
		RunInBackground: true,
	}
	executeSecret = "test-secret"
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
