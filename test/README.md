# Integration Tests

This directory contains end-to-end integration tests for the digitalhub-serverless project, demonstrating various trigger and sink combinations.

## Available Tests

### MJPEG Trigger Tests

#### 1. **mjpeg** - MJPEG to MJPEG
Basic MJPEG trigger with MJPEG sink for re-streaming processed frames.

- **Trigger**: MJPEG stream consumer
- **Sink**: MJPEG HTTP server
- **Processing Factor**: 1 (every frame)
- **Use Case**: Re-streaming camera feeds with processing applied

See [mjpeg/README.md](mjpeg/README.md) for details.

#### 2. **mjpeg-rtsp** - MJPEG to RTSP
Process MJPEG frames and stream to RTSP protocol.

- **Trigger**: MJPEG stream consumer
- **Sink**: RTSP server (via FFmpeg)
- **Processing Factor**: 1 (every frame)
- **Use Case**: Converting MJPEG streams to RTSP for better compatibility
- **Requires**: FFmpeg installed

See [mjpeg-rtsp/README.md](mjpeg-rtsp/README.md) for details.

#### 3. **mjpeg-websocket** - MJPEG to WebSocket
Forward MJPEG frames to WebSocket endpoints.

- **Trigger**: MJPEG stream consumer
- **Sink**: WebSocket client
- **Processing Factor**: 2 (every 2nd frame)
- **Use Case**: Real-time frame delivery to WebSocket-based dashboards
- **Includes**: WebSocket test server (`websocket_test_server.py`)

See [mjpeg-websocket/README.md](mjpeg-websocket/README.md) for details.

#### 4. **mjpeg-webhook** - MJPEG to Webhook
Send frame analysis results to HTTP webhooks.

- **Trigger**: MJPEG stream consumer
- **Sink**: Webhook (HTTP POST)
- **Processing Factor**: 5 (every 5th frame)
- **Use Case**: Sending frame metadata and analysis to REST APIs
- **Includes**: Webhook test server (`webhook_test_server.py`)

See [mjpeg-webhook/README.md](mjpeg-webhook/README.md) for details.

### WebSocket Trigger Tests

#### 5. **websocket_discrete** - Discrete WebSocket Processing
Process individual WebSocket messages as discrete events.

- **Trigger**: WebSocket server (discrete mode)
- **Processing**: Individual messages
- **Use Case**: Processing independent WebSocket events

See [websocket_discrete/README.md](websocket_discrete/README.md) for details.

#### 6. **websocket_stream** - Streaming WebSocket Processing
Aggregate WebSocket messages into continuous streams.

- **Trigger**: WebSocket server (stream mode)
- **Processing**: Aggregated byte streams
- **Use Case**: Processing continuous data streams over WebSocket

See [websocket_stream/README.md](websocket_stream/README.md) for details.

### Other Tests

#### 7. **extproc** - External Processor
Integration with external processing via gRPC.

See [extproc/README.md](extproc/README.md) for details.

#### 8. **python** - Python Runtime
Basic Python runtime tests and observability.

See [python/README.md](python/README.md) for details.

## Test Structure

Each test directory follows a consistent structure:

```
test/<test-name>/
├── README.md                 # Test documentation
├── run.sh                    # Script to run the test
├── <test-name>-processor.yaml  # Function configuration
├── test_handler.py           # Python handler implementation
├── _nuclio_wrapper.py        # Nuclio Python runtime wrapper
└── *_test_server.py          # Optional test servers (WebSocket, Webhook)
```

## Running Tests

### General Pattern

1. Navigate to the project root directory
2. For tests with external servers (WebSocket, Webhook):
   - First start the test server in a separate terminal
3. Run the test script:
   ```bash
   sh ./test/<test-name>/run.sh
   ```

### Prerequisites

- Go 1.21 or later
- Python 3.x with nuclio-sdk
- Additional dependencies per test:
  - **RTSP tests**: FFmpeg
  - **WebSocket tests**: `websockets` Python package
  - **Python tests**: Various packages as documented

## Configuration

Each test includes a YAML configuration file that defines:

- **Function metadata** (name, namespace)
- **Runtime** (Python, Go, etc.)
- **Handler** (entry point function)
- **Triggers** (MJPEG, WebSocket, etc.) with their attributes
- **Sinks** (optional, for output streaming)
- **Workers** (concurrency settings)

## Sink Integration Tests

The following tests demonstrate the sink abstraction integration:

| Test | Source | Destination | Data Flow |
|------|--------|-------------|-----------|
| mjpeg | MJPEG stream | MJPEG HTTP | JPEG frames → Processing → JPEG frames |
| mjpeg-rtsp | MJPEG stream | RTSP server | JPEG frames → Processing → RTSP video |
| mjpeg-websocket | MJPEG stream | WebSocket | JPEG frames → Processing → Binary messages |
| mjpeg-webhook | MJPEG stream | HTTP webhook | JPEG frames → Processing → JSON metadata |

## Development

### Adding a New Test

1. Create a new directory under `test/`
2. Add required files:
   - `README.md` - Documentation
   - `run.sh` - Execution script
   - `<name>-processor.yaml` - Function configuration
   - `test_handler.py` - Handler implementation
   - `_nuclio_wrapper.py` - Copy from another test
3. Make `run.sh` executable: `chmod +x test/<name>/run.sh`
4. Document the test in this README

### Modifying Handlers

Handlers are Python functions that:
- Receive events from triggers
- Process the data (transform, analyze, etc.)
- Return responses that may be forwarded to sinks

Example handler structure:
```python
import nuclio_sdk

def handler_serve(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    # Access event data
    data = event.body
    
    # Process data
    result = process(data)
    
    # Return response
    return context.Response(
        body=result,
        status_code=200,
        content_type="application/json"
    )
```

## Troubleshooting

### Common Issues

1. **Python import errors**
   - Ensure PYTHONPATH is set correctly in run.sh
   - Check nuclio-sdk is installed: `pip install nuclio-sdk`

2. **Port conflicts**
   - Check if ports are already in use (8080, 8081, 8554, 9090, etc.)
   - Modify port numbers in YAML configurations

3. **Connection errors**
   - Verify test servers are running before starting processor
   - Check network connectivity to external services
   - Review firewall settings

4. **Build errors**
   - Run `go build ./cmd/processor/...` to check for compilation issues
   - Ensure all dependencies are available

### Logs

Tests output logs to stdout/stderr. Look for:
- Connection status
- Frame processing logs
- Error messages and stack traces
- Performance metrics

## Related Documentation

- [Sink Package README](../pkg/processor/sink/README.md) - Detailed sink documentation
- [MJPEG Trigger](../pkg/processor/trigger/mjpeg/) - MJPEG trigger implementation
- [WebSocket Trigger](../pkg/processor/trigger/websocket/) - WebSocket trigger implementation
- [Nuclio Documentation](https://nuclio.io/docs/) - General Nuclio documentation

## Contributing

When adding new tests:
1. Follow the existing directory structure
2. Include comprehensive README documentation
3. Add test servers where appropriate
4. Update this main README with the new test
5. Ensure tests run successfully before submitting
