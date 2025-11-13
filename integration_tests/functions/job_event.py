import os
import json
import nuclio_sdk

def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    context.logger.info_with('Invoked job handler')
    output_path = os.getenv("JOB_OUTPUT_FILE")

    if not output_path:
        raise ValueError("JOB_OUTPUT_FILE environment variable not set")

    body_str = event.body.decode('utf-8')
    parsed_body = json.loads(body_str)
    response = {
        "body": parsed_body
    }

    with open(output_path, "w") as f:
        f.write(json.dumps(response))

    context.logger.info_with('Job completed', result=response)
    return response
