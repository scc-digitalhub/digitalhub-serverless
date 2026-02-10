import numpy as np
import whisper
import nuclio_sdk

model = whisper.load_model("tiny")

def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    context.logger.info_with('Invoked rtsp handler')

    # model = whisper.load_model("tiny")
    pcm_bytes = event.body
    audio = np.frombuffer(pcm_bytes, dtype=np.int16).astype(np.float32) / 32768.0

    result = model.transcribe(audio)
    # result = model.transcribe(audio, language="de")
    context.logger.info_with(len(pcm_bytes))
    context.logger.info_with(f"transcription: {result['text']}")

    return result['text']
