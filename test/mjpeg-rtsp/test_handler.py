#
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0
#
import nuclio_sdk

def handler_serve(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    """
    Process MJPEG frames and output to RTSP sink.
    
    This handler receives JPEG frames from an MJPEG stream,
    processes them (optional), and forwards them to an RTSP stream.
    """
    # Get the JPEG frame data
    frame_data = event.body
    
    # Get frame metadata
    frame_num = event.fields.get("frame_num")
    url = event.url
    
    # context.logger.info(f"Processing frame {frame_num} from {url}")
    # context.logger.info(f"Frame size: {len(frame_data)} bytes")
    
    # Optional: Process the frame with OpenCV or other libraries
    # For example:
    # import cv2
    # import numpy as np
    # nparr = np.frombuffer(frame_data, np.uint8)
    # frame = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
    # # Apply filters, detection, etc.
    # _, encoded = cv2.imencode('.jpg', frame)
    # frame_data = encoded.tobytes()
    
    # Return the frame data to be sent to RTSP sink
    return context.Response(
        body=frame_data,
        status_code=200,
        content_type="image/jpeg"
    )
