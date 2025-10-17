The integration tests require python virtual environment:

 - specify which python version is being used in yaml configuration files (e.g., runtime: python:3.10)
 - python venv used in the test files (e.g., env = append(env, "PATH="+filepath.Join(projectDir, "./venv3.10/bin")+":"+os.Getenv("PATH")))


Currently http triggers work only on the port 8080.