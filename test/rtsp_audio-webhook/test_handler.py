import json
import whisper
import nuclio_sdk
import numpy as np

model = whisper.load_model("tiny")

def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    context.logger.info_with('Invoked rtsp handler')

    pcm_bytes = event.body
    audio = np.frombuffer(pcm_bytes, dtype=np.int16).astype(np.float32) / 32768.0
    result = model.transcribe(audio)
    context.logger.info_with(len(pcm_bytes))
    context.logger.info_with(f"transcription: {result['text']}")



    payload = {"transcription": result['text'],
               "size": len(pcm_bytes)
               }

    return context.Response(
        body=json.dumps(payload),
        status_code=200,
        content_type="application/json"
    )


# import base64
# import io
# import json
# import nuclio_sdk
# from PIL import Image
# from ultralytics import YOLOWorld


# model = YOLOWorld("test/rtsp_video-webhook/yolov8s-world.pt")


# def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
#     """Minimal handler: save incoming JPEG bytes to a frames directory.

#     Returns the saved file path or an empty string on no-op/failure.
#     """
#     frame_bytes = getattr(event, "body", None)
#     if not frame_bytes:
#         if hasattr(context, "logger"):
#             try:
#                 context.logger.info_with("Empty frame received")
#             except Exception:
#                 pass
#         return ""

#     # Only accept JPEG frames (start with 0xFFD8)
#     if not frame_bytes.startswith(b"\xff\xd8"):
#         if hasattr(context, "logger"):
#             try:
#                 context.logger.info_with("Non-JPEG payload received; ignoring")
#             except Exception:
#                 pass
#         return ""
    
#     image_data = io.BytesIO(frame_bytes)
#     image = Image.open(image_data)  
#     results = model.predict(image)

    
