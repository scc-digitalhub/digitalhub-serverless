# MJPEG to RTSP Test

This test demonstrates the integration between MJPEG trigger and RTSP sink. It receives frames from an MJPEG stream, processes them through a Python handler, and streams the output to an RTSP server using native Go implementation.

## Overview

- **Trigger**: MJPEG (consumes MJPEG stream)
- **Sink**: RTSP (outputs to RTSP stream via gortsplib)
- **Processing Factor**: 1 (process every frame)
- **Use Case**: Re-streaming MJPEG camera feeds to RTSP for wider compatibility

## Prerequisites

- Python 3.x with nuclio-sdk
- No external dependencies required (RTSP server uses native Go implementation)

## Configuration

The configuration is defined in `mjpeg-rtsp-processor.yaml`:

```yaml
triggers:
  mjpeg:
    kind: mjpeg
    attributes:
      url: "http://77.222.181.11:8080/mjpg/video.mjpg"  # Source MJPEG stream
      processing_factor: 1  # Process every frame
      sink:
        kind: rtsp
        attributes:
          port: 8554        # RTSP server port (default: 8554)
          path: "/processed"  # Stream path
          type: "video"     # Stream type (video or audio)
    maxWorkers: 4
```

## Running the Test

1. From the project root, run:
   ```bash
   sh ./test/mjpeg-rtsp/run.sh
   ```

3. The processor will:
   - Connect to the MJPEG stream at the configured URL
   - Start FFmpeg in RTSP server mode (listening for connections)
   - Process frames through the Python handler
   - Stream output to connected RTSP clients

4. **Important**: The RTSP server starts immediately and waits for clients to connect. After starting the processor, connect with a player like VLC or ffplay:
   ```bash
   ffplay rtsp://localhost:7554/processed
   
   # Or with VLC
   vlc rtsp://localhost:7554/processed
   ```

   Once a client connects, the RTSP sink will start streaming frames via RTP packets.

## Handler Function

The `test_handler.py` contains a simple handler that:
- Receives JPEG frames from the MJPEG trigger
- Logs frame metadata (number, size, source URL)
- Returns the frame data (optionally processed) to the RTSP sink

You can customize the handler to:
- Apply filters or transformations with OpenCV
- Perform object detection or tracking
- Add overlays or annotations

## Troubleshooting

### Cannot connect to MJPEG source
- Check the URL is accessible
- Verify network connectivity
- Try the URL in a browser first

### RTSP stream not working / Connection refused
- The RTSP server uses native Go implementation (gortsplib)
- Check if port 7554 is available (or the port you configured)
- Verify the RTSP server started (check processor logs for "Starting RTSP sink")
- Try a different RTSP client (ffplay, VLC, mpv)
- Make sure to use `rtsp://localhost:7554/processed` (match the configured port and path)

### No video playing
- The RTSP server streams frames as they arrive from the MJPEG source
- Check that frames are being processed (look at handler logs)
- Verify the MJPEG source is providing frames

### Performance issues
- Increase `processing_factor` to process fewer frames
- Reduce the number of workers
- Check CPU and memory usage

## Technical Details

The native Go RTSP implementation:
- Uses `gortsplib` for RTSP server functionality
- Encapsulates MJPEG frames in RTP packets (RFC 2435)
- Supports both TCP and UDP transport
- No external dependencies (FFmpeg not required)
- Automatically fragments large frames into multiple RTP packets

## Files

- `run.sh` - Script to start the processor
- `mjpeg-rtsp-processor.yaml` - Function configuration
- `test_handler.py` - Python handler implementing frame processing
- `_nuclio_wrapper.py` - Nuclio Python runtime wrapper
