import os
import nuclio_sdk

def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    context.logger.info_with('Invoked job handler')

    input_path = os.getenv("JOB_INPUT_FILE")
    output_path = os.getenv("JOB_OUTPUT_FILE")

    if not input_path or not os.path.exists(input_path):
        raise FileNotFoundError(f"Input file not found: {input_path}")
    if not output_path:
        raise ValueError("JOB_OUTPUT_FILE environment variable not set")

    with open(input_path, "r") as f:
        text = f.read().strip()

    result = f"Got {text}. Job done."

    with open(output_path, "w") as f:
        f.write(result)

    context.logger.info_with('Job completed', result=result)
    return result
