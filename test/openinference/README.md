# OpenInference Trigger Test

This directory contains integration tests for the OpenInference trigger.

## Test Structure

- `test_handler.py` - Python handler that processes inference requests
- `openinference-processor.yaml` - Configuration for the test function
- `run.sh` - Script to start the processor with the test configuration
- `test_rest_client.py` - Comprehensive REST API test client
- `test_grpc_client.py` - gRPC API test client (placeholder - requires proto code generation)
- `_nuclio_wrapper.py` - Nuclio Python wrapper

## Running the Tests

### 1. Start the Test Server

From the project root directory:

```bash
cd test/openinference
chmod +x run.sh
./run.sh
```

The server will start with:
- REST API on `http://localhost:8080/v2/`
- gRPC API on `localhost:9000`

### 2. Run REST API Tests

In a separate terminal:

```bash
cd test/openinference
python3 test_rest_client.py --wait
```

## Test Coverage

### REST API Tests

1. **Health Endpoints**
   - `/v2/health/live` - Server liveness check
   - `/v2/health/ready` - Server readiness check

2. **Metadata Endpoints**
   - `/v2/models/{model_name}` - Model metadata

3. **Inference Endpoint** - `/v2/models/{model_name}/infer`
   - FP32 tensor inference
   - INT64 tensor inference
   - BYTES (string) tensor inference
   - Boolean tensor inference
   - Multiple inputs in single request
   - Inference with parameters (temperature, max_tokens, etc.)

### gRPC API Tests

The gRPC test client is a placeholder that requires Python gRPC code generation from the proto files.

To generate the gRPC code:

```bash
# Install required tools
pip install grpcio grpcio-tools

# Generate Python gRPC code
python -m grpc_tools.protoc \
    -I../../pkg/proto/inference/v2 \
    --python_out=. \
    --grpc_python_out=. \
    ../../pkg/proto/inference/v2/grpc_service.proto
```

## Manual Testing

### Using curl

```bash
# Check server is live
curl http://localhost:8080/v2/health/live

# Get model metadata
curl http://localhost:8080/v2/models/test-model

# Send inference request
curl -X POST http://localhost:8080/v2/models/test-model/infer \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-001",
    "inputs": [
      {
        "name": "input",
        "datatype": "FP32",
        "shape": [1, 3],
        "data": [1.0, 2.0, 3.0]
      }
    ]
  }'
```

### Using grpcurl

```bash
# Install grpcurl
brew install grpcurl

# List services
grpcurl -plaintext localhost:9000 list

# Call ServerLive
grpcurl -plaintext localhost:9000 inference.GRPCInferenceService/ServerLive

# Call ModelMetadata
grpcurl -plaintext -d '{"name": "test-model", "version": "1.0"}' \
  localhost:9000 inference.GRPCInferenceService/ModelMetadata

# Send inference request
grpcurl -plaintext -d '{
  "model_name": "test-model",
  "model_version": "1.0",
  "id": "test-001",
  "inputs": [
    {
      "name": "input",
      "datatype": "FP32",
      "shape": [1, 3],
      "contents": {
        "fp32_contents": [1.0, 2.0, 3.0]
      }
    }
  ]
}' localhost:9000 inference.GRPCInferenceService/ModelInfer
```

## Expected Behavior

The test handler multiplies all numeric input values by 2:
- FP32/FP64: `[1.0, 2.0, 3.0]` → `[2.0, 4.0, 6.0]`
- INT types: `[10, 20, 30]` → `[20, 40, 60]`
- BOOL: `[true, false]` → `[false, true]`
- BYTES: `["hello", "world"]` → `["HELLO", "WORLD"]`

## Configuration

The test uses the following configuration in `openinference-processor.yaml`:

```yaml
triggers:
  openinference:
    kind: openinference
    attributes:
      modelName: "test-model"
      modelVersion: "1.0"
      enableREST: true
      enableGRPC: true
      restPort: 8080
      grpcPort: 9000
      inputTensors:
        - name: "input"
          dataType: "FP32"
          shape: [1, 224, 224, 3]
        - name: "text_input"
          dataType: "BYTES"
          shape: [1]
      outputTensors:
        - name: "output_input"
          dataType: "FP32"
          shape: [1, 224, 224, 3]
        - name: "output_text_input"
          dataType: "BYTES"
          shape: [1]
    maxWorkers: 4
```

## Troubleshooting

1. **Port already in use**: Change the `restPort` or `grpcPort` in the YAML config
2. **Handler errors**: Check the logs in the terminal running `run.sh`
3. **Import errors**: Ensure `PYTHONPATH` and `NUCLIO_PYTHON_WRAPPER_PATH` are set correctly (run.sh handles this)
4. **gRPC tests**: Remember to generate the Python gRPC code first

## References

- [OpenInference Protocol](https://github.com/triton-inference-server/server/blob/main/docs/protocol/extension_openinference.md)
- [KServe V2 Inference Protocol](https://kserve.github.io/website/modelserving/data_plane/v2_protocol/)
