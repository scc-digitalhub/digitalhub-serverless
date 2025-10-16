package integrationtests

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGetRequest(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	projectDir := filepath.Dir(currentDir)

	t.Logf("Directory: %s", projectDir)

	pythonPath := filepath.Join(projectDir, "./venv3.10/bin/python3.10")
	configPath := filepath.Join(projectDir, "./integration_tests/configurations/get.yaml")
	env := os.Environ()
	env = append(env, "PYTHONPATH="+filepath.Join(projectDir, "./integration_tests/functions"))
	env = append(env, "NUCLIO_PYTHON_WRAPPER_PATH="+filepath.Join(projectDir, "./integration_tests/_nuclio_wrapper.py"))
	env = append(env, "NUCLIO_PYTHON_EXECUTABLE="+pythonPath)
	env = append(env, "PATH="+filepath.Join(projectDir, "./venv3.10/bin")+":"+os.Getenv("PATH"))
	cmd := exec.Command("go", "run", "../cmd/processor", "--config="+configPath)
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	url := "http://localhost:8080"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Unexpected status code: %d", resp.StatusCode)
	}

	actualResponse := string(body)
	expectedResponse := "Hello, world!"
	if actualResponse != expectedResponse {
		t.Fatalf("Response mismatch.\nExpected: %v\nAcutal: %v", expectedResponse, actualResponse)
	} else {
		t.Logf("\nCorrect.\nExpected: %v\nAcutal: %v", expectedResponse, actualResponse)
	}

	t.Logf("HTTP trigger responded with status: %d", resp.StatusCode)
}
