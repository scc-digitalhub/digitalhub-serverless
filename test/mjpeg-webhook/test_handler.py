#
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0
#
import nuclio_sdk
import json
import base64
from datetime import datetime

def handler_serve(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    """
    Process MJPEG frames and output to Webhook sink.
    
    This handler receives JPEG frames from an MJPEG stream,
    extracts metadata or performs analysis, and sends the results
    to a webhook endpoint as JSON.
    """
    # Get the JPEG frame data
    frame_data = event.body
    
    # Get frame metadata
    frame_num = event.fields.get("frame_num")
    url = event.url
    
    context.logger.info(f"Processing frame {frame_num} from {url}")
    context.logger.info(f"Frame size: {len(frame_data)} bytes")
    
    # Optional: Process the frame with OpenCV for object detection, etc.
    # import cv2
    # import numpy as np
    # nparr = np.frombuffer(frame_data, np.uint8)
    # frame = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
    # # Perform detection/analysis
    # objects_detected = detect_objects(frame)
    
    # Create a JSON payload with frame metadata and analysis results
    # This will be sent to the webhook endpoint
    payload = {
        "frame_number": frame_num,
        "timestamp": datetime.utcnow().isoformat() + "Z",
        "source_url": url,
        "frame_size_bytes": len(frame_data),
        "analysis": {
            "processed": True,
            # Add your analysis results here
            # "objects_detected": objects_detected,
            # "confidence": 0.95,
        },
        # Optional: Include base64-encoded thumbnail
        "thumbnail": base64.b64encode(frame_data[:1000]).decode('utf-8') if len(frame_data) > 1000 else None
    }
    
    # Return JSON payload to be sent to webhook
    return context.Response(
        body=json.dumps(payload),
        status_code=200,
        content_type="application/json"
    )
