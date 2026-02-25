# Digital Hub Serverless

[![license](https://img.shields.io/badge/license-Apache%202.0-blue)](https://github.com/scc-digitalhub/digitalhub-core/LICENSE) ![GitHub Release](https://img.shields.io/github/v/release/scc-digitalhub/digitalhub-serverless)
![Status](https://img.shields.io/badge/status-stable-gold)

Nuclio "Serverless"-based framework for Job/serverless executions compatible with DH Core. The product is a set of python images that can be used to run 
- serverless jobs in a Kubernetes cluster (``job`` trigger).
- functions as APIs in a Kubernetes cluster (based on Nuclio ``http`` trigger).
- traffic processing tasks as extensions (``ext_proc``) to Envoy proxy as a service sidecar or standalone.

## Job Trigger Configuration

Trigger of type ``job`` extends the family of available triggers with the possibility to execute the container just ones with the predefined
event defined in the trigger specification.  

## Extproc Trigger Configuration

Trigger of type ``extproc`` extends the family of available triggers with the functionality to handle envoy extProc message processing. See the corresponding
[Envoy Proxy Specification](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_proc_filter) for details of the integration configuration.

When exposed, the Envoy proxy ``ProcessingRequest`` messages are handled by the specified runtime implementation. Based on the processor pattern, the 
handler processes and transforms the incoming request, the outgoing response, or even controls whether the processing may be interrupted. More specifically, the
following scenarios are available:

- ``preprocessor`` handler: receives the Event object representing the incoming HTTP message (with URL, body, headers, etc) and returns the modified object to be passed to the upstream service.  If response with Status > 0 is returned, it is sent as immediate response. In case of processing error, the original message remains intact and passed to the upstream as is.
- ``postprocessor`` handler: receives the Event object representing the outgoing HTTP message (with URL, body, headers, etc) and returns the modified object to be passed to the client. If response with Status > 0 is returned, it is sent as  response with that status. 
- ``observeprocessor`` handler: receives both the request and response objects and may perform some processing of that without, however, altering the flow. In fact, the execution of observe processor should be considered asynchronous and its results are ignored.
- ``wrapprocessor`` handler: receives both the request and response objects and may perform some processing of ther messages. If upon request event it is necesary to prevent the propagation to the upstream service, the wrap processor should return the result and additionally append the ``X-Processing-Status`` header to signal that the processing should be terminated with the corresponding status code. This is necessary to disntinguish from the default status code returned by the processing chain. 

In order to see which processing phase is engaged, the request object is equipped with the ``processing-phase`` header with the following values:

- process request headers: 1
- process request body: 2
- process response headers: 4
- process response body: 5

The trigger configuration is defined as follows:

```yaml
    myextproctrigger:
      kind: extproc
      attributes:
        type: wrapprocessor                        
        port: 5051                                 
        gracefulShutdownTimeout: 15
        maxConcurrentStreams: 100
        processingOptions:
          logStream: false
          logPhases: false
          requestIdHeaderName: x-request-id
          bufferStreamedBodies: false
          perRequestBodyBufferBytes: 102400
          decompressBodies: true
      maxWorkers: 4  
      ...
```

where 

- ``type`` defines the processing pattern (required)
- ``port`` defines the gRPC port to expose (required)
- ``gracefulShutdownTimeout`` timeout for the processor shutdown (15 sec)
- ``maxConcurrentStreams`` for the gRPC server (100)
- ``logStream`` and ``logPhases`` define whether to log the processing information for debugging (false)
- ``requestIdHeaderName`` the name of the request id header as defined by the Envoy proxy (x-request-id)
- ``bufferStreamedBodies`` whether streamed body should be bufffered with ``perRequestBodyBufferBytes`` specifying the buffer size (false, 0)
- ``decompressBodies`` whether to decompress body for processing (true)

### Testing

To test the exproc functionality, it is possible to use the [Docker compose application](test/extproc/envoy-compose/docker-compose.yaml) including the Envoy proxy with the predefined [configuration](test/extproc/envoy-compose/envoy.yaml) and a simple upstream service. The Envoy configuration handles all the traffic with the 
extproc gRPC server outside of the compose (``host.docker.internal``, port 5051).

To run / debug the extproc processor, it is possible to run the predefined script: [test/extproc/run.sh](test/extproc/run.sh). The application
relies on a Python runtime and therefore expects a preconfigured Python runtime with the Nuclio python SDK installed.

Once you have the docker container and the application running, you can test it with the following curl command:
```bash
curl localhost:8080/resource -X POST -H 'Content-type: text/plain' -d 'hello' -s -vvv
```

## OpenInference Trigger Configuration

Trigger of type ``openinference`` extends the family of available triggers with the functionality to serve machine learning models using the [OpenInference protocol](https://github.com/openmodelio/openinference). This trigger provides standardized REST and gRPC endpoints compatible with inference serving frameworks like NVIDIA Triton, KServe, and other OpenInference-compliant systems.

The trigger exposes the following endpoints:

### REST Endpoints (HTTP/JSON)

- `GET /v2/health/live` - Server liveness check
- `GET /v2/health/ready` - Server readiness check
- `GET /v2/models/{model_name}/versions/{version}/ready` - Model readiness check
- `GET /v2/models/{model_name}/versions/{version}` - Model metadata (inputs/outputs schema)
- `POST /v2/models/{model_name}/versions/{version}/infer` - Perform inference

### gRPC Endpoints

The trigger implements the `GRPCInferenceService` from the OpenInference protocol specification:

- `ServerLive` - Server liveness check
- `ServerReady` - Server readiness check  
- `ModelReady` - Model readiness check
- `ServerMetadata` - Server metadata
- `ModelMetadata` - Model metadata with tensor definitions
- `ModelInfer` - Perform inference

### Configuration

The trigger configuration is defined as follows:

```yaml
triggers:
  myopeninferencetrigger:
    kind: openinference
    attributes:
      model_name: my-model              # Model name (default: "model")
      model_version: "1.0"               # Model version (default: "1")
      rest_port: 8080                    # REST API port (default: 8080)
      grpc_port: 9000                    # gRPC port (default: 9000)
      enable_rest: true                  # Enable REST endpoints (default: true)
      enable_grpc: true                  # Enable gRPC endpoints (default: true)
      input_tensors:                     # Input tensor definitions
        - name: input
          datatype: FP32
          shape: [1, 3, 224, 224]
      output_tensors:                    # Output tensor definitions
        - name: output
          datatype: FP32
          shape: [1, 1000]
    maxWorkers: 4
```

**Configuration Parameters:**

- `model_name` - Name of the model being served (required)
- `model_version` - Version identifier for the model (default: "1")
- `rest_port` - TCP port for REST API endpoints (default: 8080)
- `grpc_port` - TCP port for gRPC service (default: 9000)
- `enable_rest` - Enable REST API endpoints (default: true)
- `enable_grpc` - Enable gRPC service (default: true)
- `input_tensors` - Array of input tensor definitions with name, datatype, and shape
- `output_tensors` - Array of output tensor definitions with name, datatype, and shape

**Supported Data Types:**

`BOOL`, `UINT8`, `UINT16`, `UINT32`, `UINT64`, `INT8`, `INT16`, `INT32`, `INT64`, `FP16`, `FP32`, `FP64`, `BYTES`

**Handler Function:**

The handler function receives an event with the inference request and should return a response in the OpenInference format:

```python
def handler(context, event):
    # Parse input tensors from event.body
    request = json.loads(event.body)
    inputs = request["inputs"]
    
    # Perform inference
    # ... your model inference code ...
    
    # Return output tensors
    return context.Response(
        body=json.dumps({
            "model_name": request["model_name"],
            "model_version": request["model_version"],
            "outputs": [
                {
                    "name": "output",
                    "datatype": "FP32",
                    "shape": [1, 1000],
                    "data": output_data
                }
            ]
        }),
        headers={},
        content_type="application/json",
        status_code=200
    )
```

### Testing

To test the OpenInference trigger functionality, use the test suite in the [test/openinference/](test/openinference/) directory. The test suite includes:

- Python inference handler example
- REST API test client with comprehensive test scenarios
- gRPC client test examples
- Sample configuration

To run the test:

```bash
# Start the processor with the OpenInference trigger
./test/openinference/run.sh

# In another terminal, run the REST API tests
cd test/openinference
python3 test_rest_client.py

# Or run the gRPC tests
python3 test_grpc_client.py
```

Example REST API test:

```bash
# Check server liveness
curl http://localhost:8080/v2/health/live

# Get model metadata
curl http://localhost:8080/v2/models/test-model/versions/1.0

# Perform inference
curl -X POST http://localhost:8080/v2/models/test-model/versions/1.0/infer \
  -H "Content-Type: application/json" \
  -d '{
    "inputs": [{
      "name": "input",
      "datatype": "FP32",
      "shape": [1, 3],
      "data": [1.0, 2.0, 3.0]
    }]
  }'
```

## MJPEG Trigger Configuration

Trigger of type ``mjpeg`` enables processing of frames from Motion JPEG (MJPEG) video streams in real-time. MJPEG is a video compression format where each frame is individually compressed as a JPEG image and is commonly used in IP cameras, webcams, and video surveillance systems.

The MJPEG trigger connects to an MJPEG stream URL, extracts individual frames, and passes them to your function handler for processing. This enables use cases such as:

- **Video Analytics**: Real-time object detection, face recognition, motion detection
- **Surveillance**: Monitor camera feeds for security events
- **Quality Control**: Inspect manufacturing processes via camera feeds
- **Traffic Monitoring**: Analyze traffic patterns from road cameras
- **Retail Analytics**: Count customers, analyze behavior patterns

### Configuration

The trigger configuration is defined as follows:

```yaml
triggers:
  mjpeg_stream:
    kind: mjpeg
    attributes:
      url: "http://camera.example.com:8080/stream.mjpg"  # MJPEG stream URL (required)
      processing_factor: 1                                # Frame sampling (default: 1)
      sink:                                               # Optional sink configuration
        kind: rtsp                                        # Sink type (rtsp, websocket, webhook, mjpeg)
        attributes:
          port: 8554
          path: "/stream"
    maxWorkers: 4
```

**Configuration Parameters:**

- `url` - URL of the MJPEG stream to connect to (required)
  - Example: `http://192.168.1.100:8080/video.mjpg`
- `processing_factor` - Controls frame sampling to reduce processing load (default: 1)
  - Value `1`: process every frame
  - Value `2`: process every 2nd frame (50% frame drop)
  - Value `5`: process every 5th frame (80% frame drop)
  - Must be >= 1
- `sink` - Optional sink configuration for output streaming (see Sink documentation)

### Event Structure

When a frame is processed, the handler receives an event with:

- **Body**: Raw JPEG image data (bytes)
- **Content-Type**: `image/jpeg`
- **Fields**:
  - `frame_num`: Sequential frame number (int64)
  - `url`: The source MJPEG stream URL
  - `timestamp`: Frame capture timestamp

### Handler Function

Example Python handler for processing MJPEG frames:

```python
def handler(context, event):
    # Get the JPEG frame data
    frame_data = event.body
    
    # Get frame metadata
    frame_num = event.get_field("frame_num")
    url = event.get_field("url")
    
    context.logger.info(f"Processing frame {frame_num} from {url}")
    context.logger.info(f"Frame size: {len(frame_data)} bytes")
    
    # Process the frame (e.g., save, analyze with CV library)
    # ...
    
    return context.Response(
        body=frame_data,  # Return processed frame
        content_type="image/jpeg",
        status_code=200
    )
```

Example handler with image processing using OpenCV:

```python
import cv2
import numpy as np

def handler(context, event):
    # Decode JPEG frame
    nparr = np.frombuffer(event.body, np.uint8)
    img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
    
    # Perform image processing (e.g., face detection)
    gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
    faces = face_cascade.detectMultiScale(gray, 1.1, 4)
    
    # Draw rectangles around faces
    for (x, y, w, h) in faces:
        cv2.rectangle(img, (x, y), (x+w, y+h), (255, 0, 0), 2)
    
    frame_num = event.get_field("frame_num")
    context.logger.info(f"Detected {len(faces)} faces in frame {frame_num}")
    
    # Encode back to JPEG
    _, encoded = cv2.imencode('.jpg', img)
    
    return context.Response(
        body=encoded.tobytes(),
        content_type="image/jpeg",
        status_code=200
    )
```

### Behavior

**Stream Connection:**
- The trigger automatically connects to the MJPEG stream on start
- If the connection is lost, it automatically retries every 5 seconds
- The trigger continues running until explicitly stopped

**Frame Processing:**
- Frames are extracted from the multipart MIME stream
- Each frame is wrapped as a Nuclio event
- The event is submitted to an available worker for processing
- If `processing_factor > 1`, frames are dropped to reduce load

**Error Handling:**
- Connection errors trigger automatic reconnection
- Frame parsing errors are logged but don't stop the stream
- Worker allocation failures are logged (event is dropped)

### Sink Integration

The MJPEG trigger supports optional sink configuration for outputting processed frames. Available sinks include:

- **RTSP**: Stream to RTSP clients (native Go implementation using gortsplib)
- **WebSocket**: Stream to WebSocket clients
- **Webhook**: Send frames to HTTP endpoints
- **MJPEG**: Re-stream as MJPEG

Example with RTSP sink:

```yaml
triggers:
  mjpeg_camera:
    kind: mjpeg
    attributes:
      url: "http://camera.local/stream.mjpg"
      processing_factor: 1
      sink:
        kind: rtsp
        attributes:
          port: 8554
          path: "/processed"
          type: "video"
    maxWorkers: 4
```

After starting the processor, clients can connect to the RTSP stream:

```bash
ffplay rtsp://localhost:8554/processed
# or
vlc rtsp://localhost:8554/processed
```

### Testing

To test the MJPEG trigger functionality, use the test examples in the [test/](test/) directory:

- `test/mjpeg/` - Basic MJPEG stream processing
- `test/mjpeg-rtsp/` - MJPEG to RTSP streaming
- `test/mjpeg-webhook/` - MJPEG with webhook sink
- `test/mjpeg-websocket/` - MJPEG with WebSocket sink

Example test run:

```bash
# Start the MJPEG processor
./test/mjpeg-rtsp/run.sh

# In another terminal, connect to the output stream
ffplay rtsp://localhost:8554/processed
```

### Performance Considerations

- **Frame Rate**: Use `processing_factor` to control CPU/memory usage
- **Worker Pool**: Configure adequate workers to handle frame processing rate
- **Network**: Ensure stable network connection to the MJPEG source
- **Processing Time**: If processing takes longer than frame interval, increase `processing_factor`

## Development

See CONTRIBUTING for contribution instructions.

### Build container images

To build the container image, you need to:

Clone the repository and navigate to the `digitalhub-serverless` directory. The build process consists of three main steps:

- Build the processor image (modify the `Makefile` file to change the SERVERLESS_DOCKER_REPO and SERVERLESS_CACHE_REPO variable to your Docker repository, e.g., `docker.io/yourusername`)

```bash
make processor
```

- Build the base image (chooses the Python 3 version from 9, 10, 11 or 12)

```bash
docker build -t python-base-3-<ver> -f ./Dockerfile/Dockerfile-base-3-<ver> .
```

- Build the onbuild image (Modify the `Dockerfile/Dockerfile-onbuild-3-<ver>` file to change the SERVERLESS_DOCKER_REP variable to your Docker repository, e.g., `docker.io/yourusername`)

```bash
docker build -t python-onbuild-3-<ver> -f ./Dockerfile/Dockerfile-onbuild-3-<ver> .
```

- Build the runtime image  (Modify the `Dockerfile/Dockerfile-handler-3-<ver>` file to change the NUCLIO_BASE_IMAGE and NUCLIO_ONBUILD_IMAGE variables that point to the base and onbuild image you just built, e.g., `python-onbuild-3-<ver>`)

```bash

docker build -t python-runtime-3-<ver> -f ./Dockerfile/Dockerfile-handler-3-<ver> --build-arg GIT_TAG=<some-tag> .
```

### Launch container

To run the container, use the following command:

```bash
docker run -e PROJECT_NAME=<project-name> -e RUN_ID=<run-id> python-runtime-3-<ver>
```

Required environment variables:

- `PROJECT`: The name of the project
- `RUN_ID`: The ID of the run to execute

## Security Policy

The current release is the supported version. Security fixes are released together with all other fixes in each new release.

If you discover a security vulnerability in this project, please do not open a public issue.

Instead, report it privately by emailing us at digitalhub@fbk.eu. Include as much detail as possible to help us understand and address the issue quickly and responsibly.

## Contributing

To report a bug or request a feature, please first check the existing issues to avoid duplicates. If none exist, open a new issue with a clear title and a detailed description, including any steps to reproduce if it's a bug.

To contribute code, start by forking the repository. Clone your fork locally and create a new branch for your changes. Make sure your commits follow the [Conventional Commits v1.0](https://www.conventionalcommits.org/en/v1.0.0/) specification to keep history readable and consistent.

Once your changes are ready, push your branch to your fork and open a pull request against the main branch. Be sure to include a summary of what you changed and why. If your pull request addresses an issue, mention it in the description (e.g., “Closes #123”).

Please note that new contributors may be asked to sign a Contributor License Agreement (CLA) before their pull requests can be merged. This helps us ensure compliance with open source licensing standards.

We appreciate contributions and help in improving the project!

## Authors

This project is developed and maintained by **DSLab – Fondazione Bruno Kessler**, with contributions from the open source community. A complete list of contributors is available in the project’s commit history and pull requests.

For questions or inquiries, please contact: [digitalhub@fbk.eu](mailto:digitalhub@fbk.eu)

## Copyright and license

Copyright © 2025 DSLab – Fondazione Bruno Kessler and individual contributors.

This project is licensed under the Apache License, Version 2.0.
You may not use this file except in compliance with the License. Ownership of contributions remains with the original authors and is governed by the terms of the Apache 2.0 License, including the requirement to grant a license to the project.
