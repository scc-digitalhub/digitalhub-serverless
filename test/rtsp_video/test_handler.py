import os
import time
import nuclio_sdk
import io
from PIL import Image
from ultralytics import YOLOWorld


model = YOLOWorld("test/rtsp_video/yolov8s-world.pt")


def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    """Minimal handler: save incoming JPEG bytes to a frames directory.

    Returns the saved file path or an empty string on no-op/failure.
    """
    frame_bytes = getattr(event, "body", None)
    if not frame_bytes:
        if hasattr(context, "logger"):
            try:
                context.logger.info_with("Empty frame received")
            except Exception:
                pass
        return ""

    # Only accept JPEG frames (start with 0xFFD8)
    if not frame_bytes.startswith(b"\xff\xd8"):
        if hasattr(context, "logger"):
            try:
                context.logger.info_with("Non-JPEG payload received; ignoring")
            except Exception:
                pass
        return ""
    
    image_data = io.BytesIO(frame_bytes)
    image = Image.open(image_data)  
    results = model.predict(image)

    payload = {"YOLO results": results[0].summary(),
               "data": frame_bytes
               }

    context.logger.info_with(results[0].summary())
    return payload
