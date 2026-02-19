# Sink Package

## Overview

The sink package provides an abstraction for streaming worker outputs to various destinations. Sinks enable you to forward processed data from Nuclio function workers to external systems or protocols such as MJPEG streams, RTSP servers, WebSocket connections, or HTTP webhooks.

Sinks are designed to work seamlessly with triggers (especially MJPEG and WebSocket triggers) to create complete data processing pipelines where data flows from a source (trigger) through your function handler and out to a destination (sink).

## Architecture

### Sink Interface

All sinks implement the `Sink` interface:

```go
type Sink interface {
    // Start initializes and starts the sink
    Start() error

    // Stop gracefully shuts down the sink
    Stop(force bool) error

    // Write sends data to the sink
    Write(ctx context.Context, data []byte, metadata map[string]interface{}) error

    // GetKind returns the sink type (e.g., "rtsp", "mjpeg", "websocket", "webhook")
    GetKind() string

    // GetConfig returns the sink configuration
    GetConfig() map[string]interface{}
}
```

### Registry

Sinks are registered in a global registry (`RegistrySingleton`) and can be created by kind:

```go
import "github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink"

sinkInstance, err := sink.RegistrySingleton.Create(
    logger,
    "mjpeg",
    map[string]interface{}{
        "port": 8081,
        "path": "/stream",
    },
)
```

## Available Sink Types

### 1. MJPEG Sink

Streams JPEG frames over HTTP using multipart/x-mixed-replace content type.

**Configuration:**
- `port` (int, default: 8081): HTTP server port
- `path` (string, default: "/stream"): HTTP endpoint path
- `boundary` (string, default: "frame"): MIME boundary string

**Example:**
```yaml
sink:
  kind: mjpeg
  attributes:
    port: 8081
    path: "/processed"
    boundary: "frame"
```

**Use Case:** Re-streaming processed video frames from MJPEG camera sources.

### 2. RTSP Sink

Streams video or audio over RTSP protocol using native Go implementation (gortsplib).

**Configuration:**
- `port` (int, default: 8554): RTSP server port
- `path` (string, default: "/stream"): Stream path
- `type` (string, default: "video"): Stream type ("video" or "audio")
- `sample_rate` (int, default: 16000): Sample rate for audio (when type is "audio")
- `channels` (int, default: 1): Number of audio channels (when type is "audio")

**Example:**
```yaml
sink:
  kind: rtsp
  attributes:
    port: 8554
    path: "/processed"
    type: "video"
```

**Requirements:** None - uses native Go implementation with no external dependencies.

**Use Case:** Broadcasting processed video/audio streams to RTSP-compatible clients.

**Technical Details:**
- Video: Streams MJPEG frames using RTP encapsulation (RFC 2435)
- Audio: Streams PCM little-endian audio using LPCM format
- Automatically fragments large frames into multiple RTP packets
- Supports both TCP and UDP transport protocols

### 3. WebSocket Sink

Sends data to a WebSocket server endpoint.

**Configuration:**
- `url` (string, required): WebSocket server URL (ws:// or wss://)
- `messageType` (string, default: "binary"): Message type ("text" or "binary")
- `timeout` (int, default: 10): Connection timeout in seconds

**Example:**
```yaml
sink:
  kind: websocket
  attributes:
    url: "ws://processor.example.com/frames"
    messageType: "binary"
    timeout: 10
```

**Features:**
- Automatic reconnection on connection loss
- Support for both text and binary messages

**Use Case:** Forwarding processed data to WebSocket-based services or dashboards.

### 4. Webhook Sink

Sends data to HTTP endpoints via POST or other HTTP methods.

**Configuration:**
- `url` (string, required): Target HTTP endpoint
- `method` (string, default: "POST"): HTTP method
- `headers` (map, optional): Custom HTTP headers
- `timeout` (int, default: 10): Request timeout in seconds
- `maxRetries` (int, default: 3): Maximum retry attempts
- `retryDelay` (int, default: 1): Delay between retries in seconds

**Example:**
```yaml
sink:
  kind: webhook
  attributes:
    url: "https://api.example.com/data"
    method: "POST"
    headers:
      Authorization: "Bearer YOUR_TOKEN"
      Content-Type: "application/json"
    timeout: 15
    maxRetries: 3
    retryDelay: 2
```

**Features:**
- Automatic retries with configurable delay
- Custom headers support
- Configurable HTTP methods

**Use Case:** Sending processed data to REST APIs, logging services, or data warehouses.

## Using Sinks with Triggers

Sinks are configured as part of trigger configuration in your Nuclio function specification. Currently, MJPEG and WebSocket triggers support sink integration.

### MJPEG Trigger with Sink

Process frames from an MJPEG stream and forward results to a sink:

```yaml
apiVersion: nuclio.io/v1
kind: NuclioFunction
metadata:
  name: mjpeg-processor
spec:
  runtime: python:3.9
  handler: handler:process_frame
  triggers:
    mjpeg_input:
      kind: mjpeg
      maxWorkers: 4
      attributes:
        url: "http://camera.example.com/stream.mjpg"
        processing_factor: 1  # Process every frame
        sink:
          kind: mjpeg
          attributes:
            port: 8081
            path: "/processed"
```

**How it works:**
1. MJPEG trigger receives frames from the source stream
2. Frames are processed by your function handler
3. Handler response (from `nuclio.Response.Body`) is written to the sink
4. Sink metadata includes: `frame_number`, `timestamp`, `url`

### WebSocket Trigger with Sink

Process WebSocket messages and forward results:

```yaml
apiVersion: nuclio.io/v1
kind: NuclioFunction
metadata:
  name: websocket-processor
spec:
  runtime: python:3.9
  handler: handler:process_data
  triggers:
    ws_input:
      kind: websocket
      maxWorkers: 4
      attributes:
        websocket_addr: ":8080"
        is_stream: false
        processing_interval: 100
        sink:
          kind: webhook
          attributes:
            url: "https://api.example.com/events"
            method: "POST"
            headers:
              Authorization: "Bearer token123"
```

**How it works:**
1. WebSocket trigger receives messages from connected clients
2. Messages are processed by your function handler
3. Handler response is written to the sink
4. Sink metadata includes: `timestamp`
5. Response is also sent back to WebSocket client (if applicable)

## Complete Pipeline Examples

### Example 1: MJPEG to MJPEG (Frame Processing)

Stream from a camera, process frames with AI/CV, and re-stream:

```yaml
apiVersion: nuclio.io/v1
kind: NuclioFunction
metadata:
  name: object-detection
spec:
  runtime: python:3.9
  handler: handler:detect_objects
  triggers:
    camera_stream:
      kind: mjpeg
      attributes:
        url: "http://192.168.1.100/stream.mjpg"
        processing_factor: 2  # Process every 2nd frame
        sink:
          kind: mjpeg
          attributes:
            port: 8081
            path: "/detected"
```

**Python Handler:**
```python
import nuclio_sdk
import cv2
import numpy as np

def detect_objects(context, event):
    # Decode JPEG frame
    nparr = np.frombuffer(event.body, np.uint8)
    frame = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
    
    # Run object detection
    # ... your detection logic ...
    
    # Draw bounding boxes
    # ... drawing logic ...
    
    # Encode back to JPEG
    _, encoded = cv2.imencode('.jpg', frame)
    
    return nuclio_sdk.Response(
        body=encoded.tobytes(),
        content_type='image/jpeg'
    )
```

### Example 2: WebSocket to Webhook (Event Processing)

Receive sensor data via WebSocket and forward to API:

```yaml
apiVersion: nuclio.io/v1
kind: NuclioFunction
metadata:
  name: sensor-aggregator
spec:
  runtime: python:3.9
  handler: handler:aggregate
  triggers:
    sensor_input:
      kind: websocket
      attributes:
        websocket_addr: ":9090"
        is_stream: false
        processing_interval: 1000
        sink:
          kind: webhook
          attributes:
            url: "https://iot-platform.example.com/api/readings"
            headers:
              X-API-Key: "your-api-key"
            timeout: 5
            maxRetries: 3
```

**Python Handler:**
```python
import nuclio_sdk
import json

def aggregate(context, event):
    # Parse sensor data
    data = json.loads(event.body)
    
    # Process/aggregate
    processed = {
        'timestamp': data.get('timestamp'),
        'sensor_id': data.get('id'),
        'value': data.get('value'),
        'processed_at': context.platform.get_timestamp()
    }
    
    return nuclio_sdk.Response(
        body=json.dumps(processed),
        content_type='application/json'
    )
```

### Example 3: MJPEG to WebSocket (Frame Analytics)

Process video and send analytics to WebSocket dashboard:

```yaml
apiVersion: nuclio.io/v1
kind: NuclioFunction
metadata:
  name: video-analytics
spec:
  runtime: python:3.9
  handler: handler:analyze_frame
  triggers:
    video_input:
      kind: mjpeg
      attributes:
        url: "http://surveillance-cam.example.com/feed"
        processing_factor: 5  # Analyze every 5th frame
        sink:
          kind: websocket
          attributes:
            url: "ws://dashboard.example.com/analytics"
            messageType: "text"
```

## Trigger Integration Details

### Configuration Structure

In the trigger configuration, sink is specified within the trigger's `attributes` section:

```yaml
triggers:
  trigger_name:
    kind: <trigger_type>
    attributes:
      # ... trigger-specific attributes ...
      sink:
        kind: <sink_type>
        attributes:
          # ... sink-specific attributes ...
```

The trigger's configuration struct includes:

```go
type SinkConfiguration struct {
    Kind       string                 `mapstructure:"kind"`
    Attributes map[string]interface{} `mapstructure:"attributes"`
}
```

### Lifecycle Management

Triggers manage sink lifecycle automatically:

1. **Initialization**: Sink is created when trigger is initialized
2. **Start**: `sink.Start()` is called when trigger starts
3. **Processing**: `sink.Write()` is called for each processed event
4. **Stop**: `sink.Stop()` is called when trigger stops

### Data Flow

```
┌─────────────┐
│   Source    │
│  (Trigger)  │
└──────┬──────┘
       │
       │ Raw Data
       ▼
┌─────────────┐
│   Worker    │
│  (Handler)  │
└──────┬──────┘
       │
       │ Processed Data
       ▼
┌─────────────┐
│    Sink     │
│(Destination)│
└─────────────┘
```

### Metadata

Different triggers provide different metadata when writing to sinks:

**MJPEG Trigger:**
```go
metadata := map[string]interface{}{
    "frame_number": frameCount,
    "timestamp":    timestamp,
    "url":          sourceURL,
}
```

**WebSocket Trigger:**
```go
metadata := map[string]interface{}{
    "timestamp": time.Now(),
}
```

## Implementing Custom Sinks

To create a custom sink:

1. **Implement the Sink interface:**

```go
package mysink

import (
    "context"
    "github.com/nuclio/logger"
    "github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink"
)

type MySink struct {
    logger logger.Logger
    config *Configuration
}

func (s *MySink) Start() error {
    // Initialize resources
    return nil
}

func (s *MySink) Stop(force bool) error {
    // Clean up resources
    return nil
}

func (s *MySink) Write(ctx context.Context, data []byte, metadata map[string]interface{}) error {
    // Send data to destination
    return nil
}

func (s *MySink) GetKind() string {
    return "mysink"
}

func (s *MySink) GetConfig() map[string]interface{} {
    // Return configuration
    return nil
}
```

2. **Implement the Factory interface:**

```go
type factory struct{}

func (f *factory) Create(logger logger.Logger, configuration map[string]interface{}) (sink.Sink, error) {
    config := &Configuration{}
    // Parse configuration
    return &MySink{logger: logger, config: config}, nil
}

func (f *factory) GetKind() string {
    return "mysink"
}
```

3. **Register in init():**

```go
func init() {
    sink.RegistrySingleton.Register("mysink", &factory{})
}
```

4. **Import in your application:**

```go
import _ "github.com/yourorg/yourpackage/pkg/processor/sink/mysink"
```

## Testing

Test your sink implementations using the registry:

```go
import (
    "testing"
    "github.com/nuclio/zap"
    "github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink"
    _ "github.com/yourorg/yourpackage/pkg/processor/sink/mysink"
)

func TestMySink(t *testing.T) {
    logger, _ := nucliozap.NewNuclioZapTest("test")
    
    config := map[string]interface{}{
        "key": "value",
    }
    
    s, err := sink.RegistrySingleton.Create(logger, "mysink", config)
    if err != nil {
        t.Fatal(err)
    }
    
    err = s.Start()
    if err != nil {
        t.Fatal(err)
    }
    defer s.Stop(false)
    
    err = s.Write(context.Background(), []byte("test"), nil)
    if err != nil {
        t.Fatal(err)
    }
}
```

## Best Practices

1. **Error Handling**: Sinks should log errors but not crash the trigger. Failed writes should be logged and potentially retried.

2. **Buffering**: Use buffered channels for non-blocking writes when appropriate.

3. **Resource Management**: Always clean up resources (connections, goroutines, file handles) in the `Stop()` method.

4. **Configuration Validation**: Validate configuration in the factory's `Create()` method before creating the sink instance.

5. **Idempotent Operations**: Make `Start()` and `Stop()` idempotent where possible.

6. **Metadata Usage**: Leverage metadata to provide context about the data being written (timestamps, source identifiers, etc.).

7. **Testing**: Write integration tests that verify end-to-end data flow through your sink.

## Troubleshooting

### Sink not receiving data

- Verify sink is configured in trigger YAML
- Check that handler returns `nuclio.Response` with data in the `Body` field
- Review trigger logs for sink creation and start messages
- Check network connectivity for remote sinks (WebSocket, Webhook)

### Connection issues

- For WebSocket/Webhook sinks, verify network connectivity to destination
- Check firewall rules for MJPEG/RTSP sink ports
- Review timeout settings
- Check retry configuration for Webhook sinks

### Performance issues

- Adjust trigger `processing_factor` to reduce processing load
- Increase `maxWorkers` for parallel processing
- Use buffering in custom sinks to handle bursts
- Monitor sink write latencies

## See Also

- [Integration Tests](integration_test.go) - Examples of sink usage
- [MJPEG Trigger](../trigger/mjpeg/) - MJPEG trigger documentation
- [WebSocket Trigger](../trigger/websocket/) - WebSocket trigger documentation
- [Nuclio Documentation](https://nuclio.io/docs/) - General Nuclio function documentation
