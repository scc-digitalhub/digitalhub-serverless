import nuclio_sdk

def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):

    raw = event.body
    try:
        text = raw.decode("utf-8")
    except Exception:
        context.logger.error("Failed to decode body as UTF-8")
        return "decode error"
    
    context.logger.info_with(f"Discrete event received: {text}")

    return f"ACK: {text}"
