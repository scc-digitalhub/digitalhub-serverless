package integrationtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestJobEvent(t *testing.T) {
	configPath, env := setupProcessorEnv(t, "job_event.yaml")
	outputPath := filepath.Join(os.TempDir(), "job_output.txt")
	defer os.Remove(outputPath)
	t.Logf("Output file: %s", outputPath)
	env = append(env, "JOB_OUTPUT_FILE="+outputPath)
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
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	timeout := time.After(30 * time.Second)
	select {
	case err := <-done:
		if err != nil {
			t.Logf("Processor exited (expected for job trigger): %v", err)
		}
	case <-timeout:
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		t.Fatalf("Job did not complete within timeout")
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read job output file: %v", err)
	}

	actualResponse := strings.TrimSpace(string(output))
	expectedResponse := "{\"body\": {\"text\": \"Test text. Job is done.\"}}"

	if actualResponse != expectedResponse {
		t.Fatalf("Response mismatch.\nExpected: %v\nActual: %v", expectedResponse, actualResponse)
	}
	t.Logf("Job completed successfully.\nExpected: %v\nActual: %v", expectedResponse, actualResponse)
}
