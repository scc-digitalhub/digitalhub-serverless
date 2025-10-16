import nuclio_sdk

def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    context.logger.info_with('Invoked')
    return "Hello, ......"