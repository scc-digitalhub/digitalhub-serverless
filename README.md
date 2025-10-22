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

- ``preprocessor`` handler: receives the Event object representing the incoming HTTP message (with URL, body, headers, etc) and returns the modified object to be passed to the upstream service. In case of processing error, the original message remains intact and passed to the upstream as is.
- ``postprocessor`` handler: receives the Event object representing the outgoing HTTP message (with URL, body, headers, etc) and returns the modified object to be passed to the client.
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

## Development

See CONTRIBUTING for contribution instructions.

### Build container images

To build the container image, you need to:

Clone the repository and navigate to the `digitalhub-serverless` directory. The build process consists of three main steps:

- Build the processor image (modify the `Makefile` file to change the SERVERLESS_DOCKER_REPO and SERVERLESS_CACHE_REPO variable to your Docker repository, e.g., `docker.io/yourusername`)

```bash
make processor
```

- Build the onbuild image (chooses the Python version from 3.9, 3.10, or 3.11. Modify the `Dockerfile/Dockerfile-onbuild-3-<ver>` file to change the SERVERLESS_DOCKER_REP variable to your Docker repository, e.g., `docker.io/yourusername`)

```bash
docker build -t python-onbuild-3-<ver> -f ./Dockerfile/Dockerfile-onbuild-3-<ver> -e =<ver> .
```

- Build the runtime image  (Modify the `Dockerfile/Dockerfile-handler-3-<ver>` file to change the NUCLIO_ONBUILD_IMAGE variable point to the onbuild image you just built, e.g., `python-onbuild-3-<ver>`)

```bash

docker build -t python-runtime-3-<ver> -f ./Dockerfile/Dockerfile-handler-3-<ver> .
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
