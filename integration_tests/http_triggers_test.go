package integrationtests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestGetRequest(t *testing.T) {
	configPath, env := setupProcessorEnv(t, "get.yaml")
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
	for range 20 {
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
		t.Fatalf("Response mismatch.\nExpected: %v\nActual: %v", expectedResponse, actualResponse)
	} else {
		t.Logf("\nCorrect.\nExpected: %v\nActual: %v", expectedResponse, actualResponse)
	}
	t.Logf("HTTP trigger responded with status: %d", resp.StatusCode)
}

func TestPostTextRequest(t *testing.T) {
	configPath, env := setupProcessorEnv(t, "post_text.yaml")
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
	for range 20 {
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
		t.Fatalf("Response mismatch.\nExpected: %v\nActual: %v", expectedResponse, actualResponse)
	} else {
		t.Logf("\nCorrect.\nExpected: %v\nActual: %v", expectedResponse, actualResponse)
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
		t.Fatalf("Response mismatch.\nExpected: %v\nActual: %v", expectedResponse, actualResponse)
	} else {
		t.Logf("\nCorrect.\nExpected: %v\nActual: %v", expectedResponse, actualResponse)
	}
	t.Logf("HTTP trigger responded with status: %d", resp.StatusCode)
}

func TestPostJSONRequest(t *testing.T) {
	configPath, env := setupProcessorEnv(t, "post_json.yaml")
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
	for range 20 {
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
		t.Logf("\nCorrect.\nExpected: %v\nActual: %v", string(expectedPretty), string(actualPretty))
	}
	t.Logf("HTTP trigger responded with status: %d", resp.StatusCode)
}
