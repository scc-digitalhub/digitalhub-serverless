#!/usr/bin/env python3
#
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0
#
"""
gRPC client for testing OpenInference trigger.

Note: This requires the generated gRPC Python code from the proto definitions.
To generate the gRPC code, run from the project root:

    python -m grpc_tools.protoc \
        -I./pkg/proto/inference/v2 \
        --python_out=./test/openinference \
        --grpc_python_out=./test/openinference \
        ./pkg/proto/inference/v2/grpc_service.proto

Requirements:
    pip install grpcio grpcio-tools
"""

import sys
import time
import argparse

import grpc_service_pb2, grpc_service_pb2_grpc

try:
    import grpc
    # Import generated gRPC code
    # These will be generated from the proto files
    # import grpc_service_pb2
    # import grpc_service_pb2_grpc
    GRPC_AVAILABLE = True
except ImportError as e:
    GRPC_AVAILABLE = False
    print(f"Warning: gRPC not available: {e}")
    print("Install with: pip install grpcio grpcio-tools")

SERVER_ADDRESS = "localhost:9000"
MODEL_NAME = "test-model"
MODEL_VERSION = "1.0.0"

def print_section(title):
    """Print a section header"""
    print("\n" + "=" * 60)
    print(f"  {title}")
    print("=" * 60)

def test_server_live(stub):
    """Test server liveness"""
    print_section("Testing Server Live (gRPC)")
    
    # This is example code - actual imports needed
    request = grpc_service_pb2.ServerLiveRequest()
    response = stub.ServerLive(request)
    print(f"Live: {response.live}")
    assert response.live == True
    
def test_server_ready(stub):
    """Test server readiness"""
    print_section("Testing Server Ready (gRPC)")
    request = grpc_service_pb2.ServerReadyRequest()
    response = stub.ServerReady(request)
    print(f"Ready: {response.ready}")
    assert response.ready == True

def test_model_ready(stub):
    """Test model readiness"""
    print_section("Testing Model Ready (gRPC)")
    request = grpc_service_pb2.ModelReadyRequest(
        name=MODEL_NAME,
        version=MODEL_VERSION
    )
    response = stub.ModelReady(request)
    print(f"Model Ready: {response.ready}")
    assert response.ready == True

def test_server_metadata(stub):
    """Test server metadata"""
    print_section("Testing Server Metadata (gRPC)")
    request = grpc_service_pb2.ServerMetadataRequest()
    response = stub.ServerMetadata(request)
    print(f"Server Name: {response.name}")
    print(f"Server Version: {response.version}")
    assert response.name == "digitalhub-serverless"
    assert response.version == "1.0.0" 

def test_model_metadata(stub):
    """Test model metadata"""
    print_section("Testing Model Metadata (gRPC)")
    request = grpc_service_pb2.ModelMetadataRequest(
        name=MODEL_NAME,
        version=MODEL_VERSION
    )
    response = stub.ModelMetadata(request)
    print(f"Model Name: {response.name}")
    print(f"Model Version: {response.version}")
    print(f"Inputs: {len(response.inputs)} tensors")
    for inp in response.inputs:
        print(f"  - {inp.name}: {inp.datatype} {inp.shape}")
    print(f"Outputs: {len(response.outputs)} tensors")
    for out in response.outputs:
        print(f"  - {out.name}: {out.datatype} {out.shape}")
    assert response.name == MODEL_NAME
    assert response.version == MODEL_VERSION

def test_inference_fp32(stub):
    """Test inference with FP32 data"""
    print_section("Testing FP32 Inference (gRPC)")
    print("⚠️  gRPC test placeholder - requires proto code generation")

def run_grpc_tests():
    """Run all gRPC tests"""
    print("\n" + "🚀" * 30)
    print("   OpenInference gRPC API Test Suite")
    print("🚀" * 30)
    
    if not GRPC_AVAILABLE:
        print("\n⚠️  gRPC libraries not available")
        print("Install with: pip install grpcio grpcio-tools")
        return 1
    
    print("\n⚠️  gRPC test client is a placeholder")
    print("To implement full gRPC tests:")
    print("1. Generate Python code from proto files")
    print("2. Import the generated modules")
    print("3. Implement test methods using the gRPC stub")
    
    # Example of how to connect (when proto code is generated):
    try:
        channel = grpc.insecure_channel(SERVER_ADDRESS)
        stub = grpc_service_pb2_grpc.GRPCInferenceServiceStub(channel)
        
        tests = [
            lambda: test_server_live(stub),
            lambda: test_server_ready(stub),
            lambda: test_model_ready(stub),
            lambda: test_server_metadata(stub),
            lambda: test_model_metadata(stub),
            lambda: test_inference_fp32(stub),
        ]
        
        for test in tests:
            test()
        
        channel.close()
        print("\n🎉 All gRPC tests passed!")
        return 0
    except Exception as e:
        print(f"Error: {e}")
        return 1
    
    return 0

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Test OpenInference gRPC API")
    parser.add_argument("--wait", action="store_true", help="Wait for server to start")
    parser.add_argument("--server", default=SERVER_ADDRESS, help="Server address")
    args = parser.parse_args()
    
    SERVER_ADDRESS = args.server
    
    if args.wait:
        print("Waiting 5 seconds for server to start...")
        time.sleep(5)
    
    sys.exit(run_grpc_tests())
