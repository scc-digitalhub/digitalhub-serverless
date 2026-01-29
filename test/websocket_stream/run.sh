#/bin/bash

# get current dir. Should be project root
CURRENT_DIR=$(pwd)

# set PYTHONPATH
export PYTHONPATH=$CURRENT_DIR/venv3.10/lib/python3.10/site-packages
echo "PYTHONPATH: $PYTHONPATH"

# set NUCLIO_PYTHON_WRAPPER_PATH
export NUCLIO_PYTHON_WRAPPER_PATH=$CURRENT_DIR/test/websocket_stream/_nuclio_wrapper.py
echo "NUCLIO_PYTHON_WRAPPER_PATH: $NUCLIO_PYTHON_WRAPPER_PATH"

go run ./cmd/processor --config=${CURRENT_DIR}/test/websocket_stream/ws_stream-processor.yaml
