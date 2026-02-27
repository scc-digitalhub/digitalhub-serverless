# MJPEG Trigger

The MJPEG trigger enables Nuclio functions to process frames from Motion JPEG (MJPEG) video streams in real-time.

## Overview

Motion JPEG is a video compression format where each frame is individually compressed as a JPEG image. MJPEG streams are commonly used in:
- IP cameras and webcams
- Video surveillance systems
- Live streaming applications

The MJPEG trigger connects to an MJPEG stream URL, extracts individual frames, and passes them to your function for processing.

## Configuration

### Required Attributes

- **`url`** (string): The URL of the MJPEG stream to connect to
  - Example: `http://camera.example.com:8080/stream.mjpg`

### Optional Attributes

- **`processing_factor`** (int): Controls frame sampling to reduce processing load
  - Default: `1` (process every frame)
  - Value `2`: process every 2nd frame (drop 50% of frames)
  - Value `3`: process every 3rd frame (drop 66% of frames)
  - Must be >= 1

## Example Configuration

### Process Every Frame

```yaml
triggers:
  mjpeg_stream:
    kind: mjpeg
    attributes:
      url: "http://192.168.1.100:8080/video.mjpg"
      processing_factor: 1
```

### Process Every 5th Frame (80% frame drop rate)

```yaml
triggers:
  mjpeg_stream:
    kind: mjpeg
    attributes:
      url: "http://camera.local/stream.mjpg"
      processing_factor: 5
```

## Event Structure

When a frame is processed, your function receives an event with:

- **Body**: Raw JPEG image data (bytes)
- **Content-Type**: `image/jpeg`
- **Fields**:
  - `frame_num`: Sequential frame number (int64)
  - `url`: The source MJPEG stream URL
  - `timestamp`: Frame capture timestamp

### Example Handler (Python)

```python
def handler(context, event):
    # Get the JPEG frame data
    frame_data = event.body
    
    # Get frame metadata
    frame_num = event.get_field("frame_num")
    url = event.get_field("url")
    
    context.logger.info(f"Processing frame {frame_num} from {url}")
    context.logger.info(f"Frame size: {len(frame_data)} bytes")
    
    # Process the frame (e.g., save to disk, analyze with CV library, etc.)
    # ...
    
    return context.Response(
        body=f"Processed frame {frame_num}",
        status_code=200
    )
```

### Example Handler with Image Processing (Python)

```python
import cv2
import numpy as np

def handler(context, event):
    # Decode JPEG frame
    nparr = np.frombuffer(event.body, np.uint8)
    img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
    
    # Perform image processing
    gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
    faces = face_cascade.detectMultiScale(gray, 1.1, 4)
    
    frame_num = event.get_field("frame_num")
    context.logger.info(f"Detected {len(faces)} faces in frame {frame_num}")
    
    return context.Response(
        body=f"Faces detected: {len(faces)}",
        status_code=200
    )
```

## Behavior

### Stream Connection

- The trigger automatically connects to the MJPEG stream on start
- If the connection is lost, it automatically retries every 5 seconds
- The trigger continues running until explicitly stopped

### Frame Processing

- Frames are extracted from the multipart MIME stream
- Each frame is wrapped as a Nuclio event
- The event is submitted to an available worker for processing
- If `processing_factor > 1`, frames are dropped to reduce load

### Error Handling

- Connection errors trigger automatic reconnection
- Frame parsing errors are logged but don't stop the stream
- Worker allocation failures are logged (event is dropped)

## Use Cases

1. **Video Analytics**: Real-time object detection, face recognition, motion detection
2. **Surveillance**: Monitor camera feeds for security events
3. **Quality Control**: Inspect manufacturing processes via camera feeds
4. **Traffic Monitoring**: Analyze traffic patterns from road cameras
5. **Retail Analytics**: Count customers, analyze behavior patterns

## Performance Considerations

- **Frame Rate**: Use `processing_factor` to control CPU/memory usage
- **Worker Pool**: Configure adequate workers to handle frame processing rate
- **Network**: Ensure stable network connection to the MJPEG source
- **Processing Time**: If processing takes longer than frame interval, consider increasing `processing_factor`

## Limitations

- Only supports MJPEG streams (not H.264, RTSP, etc.)
- One trigger instance connects to one stream URL
- Frames are processed asynchronously - order is not guaranteed if worker pool > 1

## Implementation Details

### Files

- `factory.go`: Trigger factory implementation for registration
- `event.go`: Event structure wrapping MJPEG frames
- `trigger.go`: Main trigger logic for stream connection and frame processing
- `types.go`: Configuration structure and validation

### Key Components

1. **Stream Connection**: Uses `http.Client` to connect to MJPEG URL
2. **Frame Extraction**: Parses multipart MIME boundary and headers
3. **Frame Processing**: Submits frames to worker pool with configured sampling factor
4. **Error Recovery**: Automatic reconnection on connection failures
