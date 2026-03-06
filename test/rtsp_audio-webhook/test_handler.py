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
