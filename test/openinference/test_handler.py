#
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0
#
import nuclio_sdk
import json
import numpy as np

def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    """
    Handler for OpenInference requests.
    Processes inference requests and returns predictions.
    """
    try:
        # Parse the inference request
        request = event.body # json.loads(event.body)
        context.logger.info(f"Received inference request: {request.get('id', 'no-id')}")
        
        # Extract model information from headers
        model_name = event.get_header("X-Model-Name") or "unknown"
        model_version = event.get_header("X-Model-Version") or "unknown"
        context.logger.info(f"Model: {model_name}, Version: {model_version}")
        
        # Process inputs
        inputs = request.get("inputs", [])
        context.logger.info(f"Number of inputs: {len(inputs)}")
        
        outputs = []
        for input_tensor in inputs:
            name = input_tensor.get("name", "unknown")
            datatype = input_tensor.get("datatype", "FP32")
            shape = input_tensor.get("shape", [])
            data = input_tensor.get("data", [])
            
            context.logger.info(f"Processing input: {name}, shape: {shape}, datatype: {datatype}")
            
            # Simple processing: for demo, multiply all values by 2
            if datatype in ["FP32", "FP64"]:
                processed_data = [x * 2.0 for x in data]
            elif datatype in ["INT8", "INT16", "INT32", "INT64"]:
                processed_data = [int(x * 2) for x in data]
            elif datatype in ["UINT8", "UINT16", "UINT32", "UINT64"]:
                processed_data = [int(abs(x * 2)) for x in data]
            elif datatype == "BOOL":
                processed_data = [not x for x in data]
            elif datatype == "BYTES":
                processed_data = [str(x).upper() for x in data]
            else:
                processed_data = data
            
            # Create output tensor
            output_tensor = {
                "name": f"output_{name}",
                "datatype": datatype,
                "shape": shape,
                "data": processed_data
            }
            outputs.append(output_tensor)
        
        # Build response
        response = {
            "id": request.get("id", ""),
            "model_name": model_name,
            "model_version": model_version,
            "outputs": outputs
        }
        
        # Add parameters from request if present
        if "parameters" in request:
            response["parameters"] = request["parameters"]
        
        context.logger.info(f"Returning {len(outputs)} outputs")
        
        return context.Response(
            body=json.dumps(response),
            headers={
                "Content-Type": "application/json"
            },
            status_code=200
        )
    
    except Exception as e:
        context.logger.error(f"Error processing request: {str(e)}")
        error_response = {
            "error": str(e)
        }
        return context.Response(
            body=json.dumps(error_response),
            headers={
                "Content-Type": "application/json"
            },
            status_code=500
        )
