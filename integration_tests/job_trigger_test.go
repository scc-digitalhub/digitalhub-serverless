package integrationtests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestJob(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	projectDir := filepath.Dir(currentDir)

	pythonPath := filepath.Join(projectDir, "./venv3.10/bin/python3.10")
	configPath := filepath.Join(projectDir, "./integration_tests/configurations/job.yaml")
	env := os.Environ()
	env = append(env, "PYTHONPATH="+filepath.Join(projectDir, "./integration_tests/functions"))
	env = append(env, "NUCLIO_PYTHON_WRAPPER_PATH="+filepath.Join(projectDir, "./integration_tests/_nuclio_wrapper.py"))
	env = append(env, "NUCLIO_PYTHON_EXECUTABLE="+pythonPath)
	env = append(env, "PATH="+filepath.Join(projectDir, "./venv3.10/bin")+":"+os.Getenv("PATH"))

	inputPath := filepath.Join(os.TempDir(), "job_input.txt")
	outputPath := filepath.Join(os.TempDir(), "job_output.txt")

	err = os.WriteFile(inputPath, []byte("job text"), 0644)
	if err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}
	defer os.Remove(inputPath)
	defer os.Remove(outputPath)

	t.Logf("Input file: %s", inputPath)
	t.Logf("Output file: %s", outputPath)

	env = append(env, "JOB_INPUT_FILE="+inputPath)
	env = append(env, "JOB_OUTPUT_FILE="+outputPath)

	cmd := exec.Command("go", "run", "../cmd/processor", "--config="+configPath)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdOut, stdErr bytes.Buffer
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

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
		t.Logf("Processor stdout: %s", stdOut.String())
		t.Logf("Processor stderr: %s", stdErr.String())
		t.Fatalf("Failed to read job output file: %v", err)
	}

	actualResponse := strings.TrimSpace(string(output))
	expectedResponse := "Got job text. Job done."

	if actualResponse != expectedResponse {
		t.Logf("Processor stdout: %s", stdOut.String())
		t.Logf("Processor stderr: %s", stdErr.String())
		t.Fatalf("Response mismatch.\nExpected: %v\nActual: %v", expectedResponse, actualResponse)
	}

	t.Logf("Job completed successfully.\nExpected: %v\nActual: %v", expectedResponse, actualResponse)
}
