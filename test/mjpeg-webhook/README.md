# MJPEG to Webhook Test

This test demonstrates the integration between MJPEG trigger and Webhook sink. It receives frames from an MJPEG stream, processes them through a Python handler, and sends the results to an HTTP webhook endpoint.

## Overview

- **Trigger**: MJPEG (consumes MJPEG stream)
- **Sink**: Webhook (HTTP POST to endpoint)
- **Processing Factor**: 5 (process every 5th frame)
- **Use Case**: Sending frame analysis results to REST APIs, logging services, or data pipelines

## Prerequisites

- Python 3.x with nuclio-sdk

## Configuration

The configuration is defined in `mjpeg-webhook-processor.yaml`:

```yaml
triggers:
  mjpeg:
    kind: mjpeg
    attributes:
      url: "http://77.222.181.11:8080/mjpg/video.mjpg"  # Source MJPEG stream
      processing_factor: 5  # Process every 5th frame
      sink:
        kind: webhook
        attributes:
          url: "http://localhost:8888/webhook"  # Webhook endpoint
          method: "POST"                        # HTTP method
          headers:                              # Custom headers
            Content-Type: "application/json"
            X-API-Key: "test-api-key-12345"
          timeout: 10         # Request timeout (seconds)
          maxRetries: 3       # Retry attempts on failure
          retryDelay: 1       # Delay between retries (seconds)
    maxWorkers: 4
```

## Running the Test

### Step 1: Start the Webhook Test Server

In a separate terminal, run the webhook server that will receive frame data:

```bash
python3 ./test/mjpeg-webhook/webhook_test_server.py
```

You should see:
```
Starting Webhook server on http://localhost:8888/webhook
API Key: test-api-key-12345
Waiting for requests...
Press Ctrl+C to stop
```

### Step 2: Start the Processor

From the project root, run:

```bash
sh ./test/mjpeg-webhook/run.sh
```

The processor will:
- Connect to the MJPEG stream at the configured URL
- Process every 5th frame through the Python handler
- Send frame metadata and analysis results to the webhook endpoint

### Step 3: Observe the Output

The webhook test server will log received requests:
```
============================================================
Request #1 received
============================================================
Frame Number: 5
Timestamp: 2026-02-19T10:30:45.123456Z
Source URL: http://77.222.181.11:8080/mjpg/video.mjpg
Frame Size: 45823 bytes
Analysis: {
  "processed": true
}
Thumbnail: 68 chars (base64)
```

Press Ctrl+C to stop either the server or processor.

## Handler Function

The `test_handler.py` contains a handler that:
- Receives JPEG frames from the MJPEG trigger
- Logs frame metadata (number, size, source URL)
- Creates a JSON payload with frame analysis results
- Returns the JSON to be sent to the webhook

The handler demonstrates:
- Creating structured JSON payloads
- Including frame metadata (number, timestamp, size)
- Adding analysis results (can be customized for object detection, etc.)
- Optionally including base64-encoded thumbnails

You can customize the handler to:
- Perform object detection or classification
- Extract features or statistics from frames
- Include full frame data or just metadata
- Send alerts or notifications

## Webhook Test Server

The `webhook_test_server.py` is a simple HTTP server that:
- Listens on http://localhost:8888/webhook
- Accepts POST requests with JSON payloads
- Validates the API key header
- Logs received frame data and analysis results
- Returns success/error responses

You can modify it to:
- Store data in a database
- Forward to another service
- Trigger actions based on analysis results
- Send notifications

## Troubleshooting

### Connection refused to webhook
- Ensure the webhook test server is running first
- Check if port 8888 is available
- Verify the URL in the configuration matches the server

### Cannot connect to MJPEG source
- Check the URL is accessible
- Verify network connectivity
- Try the URL in a browser first

### 400 Bad Request errors
- Check JSON format in handler response
- Verify Content-Type header is set correctly
- Review webhook server logs for parsing errors

### Webhook timeouts
- Increase timeout setting in configuration
- Check webhook server performance
- Verify network connectivity

### Authentication failures
- Verify API key matches in both configuration and server
- Check header names are correct

## Files

- `run.sh` - Script to start the processor
- `mjpeg-webhook-processor.yaml` - Function configuration
- `test_handler.py` - Python handler implementing frame processing and JSON creation
- `webhook_test_server.py` - HTTP webhook server for testing (receive POST requests)
- `_nuclio_wrapper.py` - Nuclio Python runtime wrapper

## Advanced Usage

### Object Detection Results

Modify the handler to include object detection results:

```python
import cv2
import numpy as np
import json

def handler_serve(context, event):
    frame_data = event.body
    frame_num = event.fields.get("frame_num")
    
    # Decode and process frame
    nparr = np.frombuffer(frame_data, np.uint8)
    frame = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
    
    # Run object detection (example)
    objects = detect_objects(frame)  # Your detection function
    
    payload = {
        "frame_number": frame_num,
        "timestamp": context.platform.get_timestamp(),
        "detections": [
            {
                "class": obj["class"],
                "confidence": obj["confidence"],
                "bbox": obj["bbox"]
            }
            for obj in objects
        ],
        "object_count": len(objects)
    }
    
    return context.Response(
        body=json.dumps(payload),
        content_type="application/json"
    )
```

### Custom Authentication

Add custom authentication headers:

```yaml
sink:
  kind: webhook
  attributes:
    url: "https://api.example.com/events"
    headers:
      Authorization: "Bearer YOUR_JWT_TOKEN"
      X-Custom-Header: "custom-value"
```

### Error Handling

The webhook sink automatically retries failed requests based on `maxRetries` and `retryDelay` settings. Adjust these based on your webhook's reliability and response time.
