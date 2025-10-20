package integrationtests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestGetRequest(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	projectDir := filepath.Dir(currentDir)

	pythonPath := filepath.Join(projectDir, "./venv3.10/bin/python3.10")
	configPath := filepath.Join(projectDir, "./integration_tests/configurations/get.yaml")
	env := os.Environ()
	env = append(env, "PYTHONPATH="+filepath.Join(projectDir, "./integration_tests/functions"))
	env = append(env, "NUCLIO_PYTHON_WRAPPER_PATH="+filepath.Join(projectDir, "./integration_tests/_nuclio_wrapper.py"))
	env = append(env, "NUCLIO_PYTHON_EXECUTABLE="+pythonPath)
	env = append(env, "PATH="+filepath.Join(projectDir, "./venv3.10/bin")+":"+os.Getenv("PATH"))
	cmd := exec.Command("go", "run", "../cmd/processor", "--config="+configPath)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			cmd.Wait()
		}
	}()

	url := "http://localhost:8080"

	ready := false
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
	}
	if !ready {
		cmd.Process.Kill()
		t.Fatalf("Processor did not start within timeout")
	}

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

func TestPostTextRequest(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	projectDir := filepath.Dir(currentDir)

	pythonPath := filepath.Join(projectDir, "./venv3.10/bin/python3.10")
	configPath := filepath.Join(projectDir, "./integration_tests/configurations/post_text.yaml")
	env := os.Environ()
	env = append(env, "PYTHONPATH="+filepath.Join(projectDir, "./integration_tests/functions"))
	env = append(env, "NUCLIO_PYTHON_WRAPPER_PATH="+filepath.Join(projectDir, "./integration_tests/_nuclio_wrapper.py"))
	env = append(env, "NUCLIO_PYTHON_EXECUTABLE="+pythonPath)
	env = append(env, "PATH="+filepath.Join(projectDir, "./venv3.10/bin")+":"+os.Getenv("PATH"))
	cmd := exec.Command("go", "run", "../cmd/processor", "--config="+configPath)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			cmd.Wait()
		}
	}()

	url := "http://localhost:8080"

	ready := false
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
	}
	if !ready {
		cmd.Process.Kill()
		t.Fatalf("Processor did not start within timeout")
	}

	postData := "world"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(postData))
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

	postData = "John"
	req, err = http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(postData))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client = &http.Client{Timeout: 5 * time.Second}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Unexpected status code: %d", resp.StatusCode)
	}

	actualResponse = string(body)
	expectedResponse = "Hello, John!"
	if actualResponse != expectedResponse {
		t.Fatalf("Response mismatch.\nExpected: %v\nAcutal: %v", expectedResponse, actualResponse)
	} else {
		t.Logf("\nCorrect.\nExpected: %v\nAcutal: %v", expectedResponse, actualResponse)
	}

	t.Logf("HTTP trigger responded with status: %d", resp.StatusCode)
}

func TestPostJSONRequest(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	projectDir := filepath.Dir(currentDir)

	pythonPath := filepath.Join(projectDir, "./venv3.10/bin/python3.10")
	configPath := filepath.Join(projectDir, "./integration_tests/configurations/post_json.yaml")
	env := os.Environ()
	env = append(env, "PYTHONPATH="+filepath.Join(projectDir, "./integration_tests/functions"))
	env = append(env, "NUCLIO_PYTHON_WRAPPER_PATH="+filepath.Join(projectDir, "./integration_tests/_nuclio_wrapper.py"))
	env = append(env, "NUCLIO_PYTHON_EXECUTABLE="+pythonPath)
	env = append(env, "PATH="+filepath.Join(projectDir, "./venv3.10/bin")+":"+os.Getenv("PATH"))
	cmd := exec.Command("go", "run", "../cmd/processor", "--config="+configPath)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			cmd.Wait()
		}
	}()

	url := "http://localhost:8080"

	ready := false
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
	}
	if !ready {
		cmd.Process.Kill()
		t.Fatalf("Processor did not start within timeout")
	}

	postData := map[string]any{"id": 0, "country": "Italy"}
	jsonData, err := json.Marshal(postData)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewBuffer(jsonData))
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
	expectedResponse := "{\"body\":{\"id\": 0, \"country\":\"Italy\"}}"

	var expectedObj, actualObj map[string]any

	if err := json.Unmarshal([]byte(expectedResponse), &expectedObj); err != nil {
		t.Fatalf("Failed to unmarshal expected JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(actualResponse), &actualObj); err != nil {
		t.Fatalf("Failed to unmarshal actual JSON: %v\nResponse: %s", err, actualResponse)
	}
	expectedPretty, _ := json.MarshalIndent(expectedObj, "", "  ")
	actualPretty, _ := json.MarshalIndent(actualObj, "", "  ")
	if !reflect.DeepEqual(expectedObj, actualObj) {
		t.Fatalf("Response mismatch.\nExpected:\n%s\nActual:\n%s", string(expectedPretty), string(actualPretty))
	} else {
		t.Logf("\nCorrect.\nExpected: %v\nAcutal: %v", string(expectedPretty), string(actualPretty))
	}

	t.Logf("HTTP trigger responded with status: %d", resp.StatusCode)
}
