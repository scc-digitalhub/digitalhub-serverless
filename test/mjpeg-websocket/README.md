# MJPEG to WebSocket Test

This test demonstrates the integration between MJPEG trigger and WebSocket sink. It receives frames from an MJPEG stream, processes them through a Python handler, and forwards the output to a WebSocket endpoint.

## Overview

- **Trigger**: MJPEG (consumes MJPEG stream)
- **Sink**: WebSocket (sends to WebSocket server)
- **Processing Factor**: 2 (process every 2nd frame)
- **Use Case**: Real-time frame forwarding to WebSocket-based dashboards or processors

## Prerequisites

- Python 3.x with nuclio-sdk
- websockets library for the test server:
  ```bash
  pip install websockets
  ```

## Configuration

The configuration is defined in `mjpeg-websocket-processor.yaml`:

```yaml
triggers:
  mjpeg:
    kind: mjpeg
    attributes:
      url: "http://77.222.181.11:8080/mjpg/video.mjpg"  # Source MJPEG stream
      processing_factor: 2  # Process every 2nd frame
      sink:
        kind: websocket
        attributes:
          url: "ws://localhost:9090/frames"  # WebSocket server endpoint
          messageType: "binary"              # Send as binary messages
          timeout: 10                         # Connection timeout (seconds)
    maxWorkers: 4
```

## Running the Test

### Step 1: Start the WebSocket Test Server

In a separate terminal, run the WebSocket server that will receive frames:

```bash
python3 ./test/mjpeg-websocket/websocket_test_server.py
```

You should see:
```
Starting WebSocket server on ws://localhost:9090/frames
Waiting for connections...
```

### Step 2: Start the Processor

From the project root, run:

```bash
sh ./test/mjpeg-websocket/run.sh
```

The processor will:
- Connect to the MJPEG stream at the configured URL
- Process frames through the Python handler
- Send frame data to the WebSocket server

### Step 3: Observe the Output

The WebSocket test server will log received frames:
```
Client connected from ('127.0.0.1', 52847)
Received frame 1: 45823 bytes
Received frame 2: 46102 bytes
Received frame 3: 45998 bytes
...
```

Press Ctrl+C to stop either the server or processor.

## Handler Function

The `test_handler.py` contains a handler that:
- Receives JPEG frames from the MJPEG trigger
- Logs frame metadata (number, size, source URL)
- Optionally processes frames with OpenCV
- Returns frame data to be sent to the WebSocket sink

You can customize the handler to:
- Add timestamps or overlays to frames
- Perform object detection or tracking
- Send different data formats (text, JSON with base64-encoded images, etc.)

## WebSocket Test Server

The `websocket_test_server.py` is a simple WebSocket server that:
- Listens on ws://localhost:9090/frames
- Accepts binary messages (JPEG frames)
- Logs received frame count and size
- Optionally can save frames to disk (uncomment code in the script)

You can modify it to:
- Display frames in real-time
- Forward frames to another service
- Perform additional processing

## Troubleshooting

### WebSocket connection refused
- Ensure the WebSocket test server is running first
- Check if port 9090 is available
- Verify the URL in the configuration matches the server

### Cannot connect to MJPEG source
- Check the URL is accessible
- Verify network connectivity
- Try the URL in a browser first

### Performance issues
- Increase `processing_factor` to process fewer frames
- Reduce the number of workers
- Check network latency to WebSocket server

### WebSocket disconnects
- Check timeout settings
- Verify network stability
- Review logs for error messages

## Files

- `run.sh` - Script to start the processor
- `mjpeg-websocket-processor.yaml` - Function configuration
- `test_handler.py` - Python handler implementing frame processing
- `websocket_test_server.py` - WebSocket server for testing (receive frames)
- `_nuclio_wrapper.py` - Nuclio Python runtime wrapper

## Advanced Usage

### Sending JSON Instead of Binary

Modify the handler to send JSON with metadata:

```python
import json
import base64

def handler_serve(context, event):
    frame_data = event.body
    
    payload = {
        "frame_number": event.fields.get("frame_num"),
        "timestamp": context.platform.get_timestamp(),
        "data": base64.b64encode(frame_data).decode('utf-8')
    }
    
    return context.Response(
        body=json.dumps(payload),
        content_type="application/json"
    )
```

And update the sink configuration:
```yaml
sink:
  kind: websocket
  attributes:
    url: "ws://localhost:9090/frames"
    messageType: "text"  # Changed to text
```
