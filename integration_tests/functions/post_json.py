import json
import nuclio_sdk

def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    context.logger.info_with('Invoked')

    body_str = event.body.decode('utf-8')
    parsed_body = json.loads(body_str)
    response = {
        "body": parsed_body
    }
    return response