package integrationtests

import (
	"os"
	"path/filepath"
	"testing"
)

func setupProcessorEnv(t *testing.T, configFile string) (string, []string) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	projectDir := filepath.Dir(currentDir)
	pythonPath := filepath.Join(projectDir, "./venv3.10/bin/python3.10")
	configPath := filepath.Join(projectDir, "./integration_tests/configurations/"+configFile)

	env := os.Environ()
	env = append(env, "PYTHONPATH="+filepath.Join(projectDir, "./integration_tests/functions"))
	env = append(env, "NUCLIO_PYTHON_WRAPPER_PATH="+filepath.Join(projectDir, "./integration_tests/_nuclio_wrapper.py"))
	env = append(env, "NUCLIO_PYTHON_EXECUTABLE="+pythonPath)
	env = append(env, "PATH="+filepath.Join(projectDir, "./venv3.10/bin")+":"+os.Getenv("PATH"))

	return configPath, env
}
