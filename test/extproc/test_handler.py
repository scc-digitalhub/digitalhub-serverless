import nuclio_sdk

def handler_serve(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    context.logger.info_with('Invoked', event=event)
    return "Hello, from Nuclio :]"