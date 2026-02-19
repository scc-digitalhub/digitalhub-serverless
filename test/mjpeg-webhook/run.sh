#!/bin/bash

# get current dir. Should be project root
CURRENT_DIR=$(pwd)

# set PYTHONPATH
export PYTHONPATH=$CURRENT_DIR/test/mjpeg-webhook
echo "PYTHONPATH: $PYTHONPATH"

# set NUCLIO_PYTHON_WRAPPER_PATH
export NUCLIO_PYTHON_WRAPPER_PATH=$CURRENT_DIR/test/mjpeg-webhook/_nuclio_wrapper.py
echo "NUCLIO_PYTHON_WRAPPER_PATH: $NUCLIO_PYTHON_WRAPPER_PATH"

go run ./cmd/processor --config=${CURRENT_DIR}/test/mjpeg-webhook/mjpeg-webhook-processor.yaml
