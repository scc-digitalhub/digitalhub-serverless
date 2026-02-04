#
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0
#
import nuclio_sdk

def handler_serve(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    # Get the JPEG frame data
    frame_data = event.body
    
    # Get frame metadata
    frame_num = event.get_field("frame_num")
    url = event.get_field("url")
    
    context.logger.info(f"Processing frame {frame_num} from {url}")
    context.logger.info(f"Frame size: {len(frame_data)} bytes")
    
    # Process the frame (e.g., save to disk, analyze with CV library, etc.)
    # ...
    
    return context.Response(
        body=f"Processed frame {frame_num}",
        status_code=200
    )