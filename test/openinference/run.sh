#!/bin/bash

# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

# Get current dir. Should be project root
CURRENT_DIR=$(pwd)

# Set PYTHONPATH
export PYTHONPATH=$CURRENT_DIR/test/openinference
echo "PYTHONPATH: $PYTHONPATH"

# Set NUCLIO_PYTHON_WRAPPER_PATH
export NUCLIO_PYTHON_WRAPPER_PATH=$CURRENT_DIR/test/openinference/_nuclio_wrapper.py
echo "NUCLIO_PYTHON_WRAPPER_PATH: $NUCLIO_PYTHON_WRAPPER_PATH"

echo "Starting OpenInference trigger test..."
echo "REST API will be available at: http://localhost:8080/v2/"
echo "gRPC API will be available at: localhost:9000"
echo ""
echo "Test endpoints:"
echo "  - Health: curl http://localhost:8080/v2/health/live"
echo "  - Metadata: curl http://localhost:8080/v2/models/test-model"
echo ""

go run ./cmd/processor --config=${CURRENT_DIR}/test/openinference/openinference-processor.yaml
