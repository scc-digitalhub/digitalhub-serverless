#!/usr/bin/env python3
#
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0
#
"""
REST client for testing OpenInference trigger.
"""
import requests
import json
import sys
import time

BASE_URL = "http://localhost:8080/v2"
MODEL_NAME = "test-model"

def print_section(title):
    """Print a section header"""
    print("\n" + "=" * 60)
    print(f"  {title}")
    print("=" * 60)

def test_server_live():
    """Test server liveness endpoint"""
    print_section("Testing Server Live")
    response = requests.get(f"{BASE_URL}/health/live")
    print(f"Status Code: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    assert response.json()["live"] == True
    print("✓ Server is live")

def test_server_ready():
    """Test server readiness endpoint"""
    print_section("Testing Server Ready")
    response = requests.get(f"{BASE_URL}/health/ready")
    print(f"Status Code: {response.status_code}")
    print(f"Response: {response.json()}")
    assert response.status_code == 200
    assert response.json()["ready"] == True
    print("✓ Server is ready")

def test_model_metadata():
    """Test model metadata endpoint"""
    print_section("Testing Model Metadata")
    response = requests.get(f"{BASE_URL}/models/{MODEL_NAME}")
    print(f"Status Code: {response.status_code}")
    data = response.json()
    print(f"Model Name: {data['name']}")
    print(f"Model Version: {data['versions']}")
    print(f"Inputs: {len(data['inputs'])} tensors")
    for inp in data['inputs']:
        print(f"  - {inp['name']}: {inp['datatype']} {inp['shape']}")
    print(f"Outputs: {len(data['outputs'])} tensors")
    for out in data['outputs']:
        print(f"  - {out['name']}: {out['datatype']} {out['shape']}")
    assert response.status_code == 200
    assert data["name"] == MODEL_NAME
    print("✓ Model metadata retrieved")

def test_inference_fp32():
    """Test inference with FP32 data"""
    print_section("Testing FP32 Inference")
    
    # Create a simple FP32 tensor (1x3)
    request = {
        "id": "test-fp32-001",
        "inputs": [
            {
                "name": "input",
                "datatype": "FP32",
                "shape": [1, 3],
                "data": [1.0, 2.0, 3.0]
            }
        ]
    }
    
    print(f"Request: {json.dumps(request, indent=2)}")
    
    response = requests.post(
        f"{BASE_URL}/models/{MODEL_NAME}/infer",
        json=request,
        headers={"Content-Type": "application/json"}
    )
    
    print(f"Status Code: {response.status_code}")
    data = response.json()
    print(f"Response: {json.dumps(data, indent=2)}")
    
    assert response.status_code == 200
    assert data["id"] == "test-fp32-001"
    assert len(data["outputs"]) > 0
    print("✓ FP32 inference completed")

def test_inference_int64():
    """Test inference with INT64 data"""
    print_section("Testing INT64 Inference")
    
    request = {
        "id": "test-int64-001",
        "inputs": [
            {
                "name": "input",
                "datatype": "INT64",
                "shape": [1, 5],
                "data": [10, 20, 30, 40, 50]
            }
        ]
    }
    
    print(f"Request: {json.dumps(request, indent=2)}")
    
    response = requests.post(
        f"{BASE_URL}/models/{MODEL_NAME}/infer",
        json=request
    )
    
    print(f"Status Code: {response.status_code}")
    data = response.json()
    print(f"Response: {json.dumps(data, indent=2)}")
    
    assert response.status_code == 200
    print("✓ INT64 inference completed")

def test_inference_bytes():
    """Test inference with BYTES data"""
    print_section("Testing BYTES Inference")
    
    request = {
        "id": "test-bytes-001",
        "inputs": [
            {
                "name": "text_input",
                "datatype": "BYTES",
                "shape": [1, 2],
                "data": ["hello", "world"]
            }
        ]
    }
    
    print(f"Request: {json.dumps(request, indent=2)}")
    
    response = requests.post(
        f"{BASE_URL}/models/{MODEL_NAME}/infer",
        json=request
    )
    
    print(f"Status Code: {response.status_code}")
    data = response.json()
    print(f"Response: {json.dumps(data, indent=2)}")
    
    assert response.status_code == 200
    print("✓ BYTES inference completed")

def test_inference_with_parameters():
    """Test inference with parameters"""
    print_section("Testing Inference with Parameters")
    
    request = {
        "id": "test-params-001",
        "parameters": {
            "temperature": 0.7,
            "max_tokens": 100,
            "top_p": 0.9
        },
        "inputs": [
            {
                "name": "input",
                "datatype": "FP32",
                "shape": [1, 2],
                "data": [0.5, 0.8]
            }
        ]
    }
    
    print(f"Request: {json.dumps(request, indent=2)}")
    
    response = requests.post(
        f"{BASE_URL}/models/{MODEL_NAME}/infer",
        json=request
    )
    
    print(f"Status Code: {response.status_code}")
    data = response.json()
    print(f"Response: {json.dumps(data, indent=2)}")
    
    assert response.status_code == 200
    print("✓ Inference with parameters completed")

def test_inference_multiple_inputs():
    """Test inference with multiple inputs"""
    print_section("Testing Multiple Inputs Inference")
    
    request = {
        "id": "test-multi-001",
        "inputs": [
            {
                "name": "input1",
                "datatype": "FP32",
                "shape": [1, 2],
                "data": [1.0, 2.0]
            },
            {
                "name": "input2",
                "datatype": "INT64",
                "shape": [1, 3],
                "data": [10, 20, 30]
            },
            {
                "name": "input3",
                "datatype": "BOOL",
                "shape": [1, 2],
                "data": [True, False]
            }
        ]
    }
    
    print(f"Request: {json.dumps(request, indent=2)}")
    
    response = requests.post(
        f"{BASE_URL}/models/{MODEL_NAME}/infer",
        json=request
    )
    
    print(f"Status Code: {response.status_code}")
    data = response.json()
    print(f"Response: {json.dumps(data, indent=2)}")
    
    assert response.status_code == 200
    assert len(data["outputs"]) == len(request["inputs"])
    print("✓ Multiple inputs inference completed")

def run_all_tests():
    """Run all REST tests"""
    print("\n" + "🚀" * 30)
    print("   OpenInference REST API Test Suite")
    print("🚀" * 30)
    
    tests = [
        test_server_live,
        test_server_ready,
        test_model_metadata,
        test_inference_fp32,
        test_inference_int64,
        test_inference_bytes,
        test_inference_with_parameters,
        test_inference_multiple_inputs,
    ]
    
    passed = 0
    failed = 0
    
    for test in tests:
        try:
            test()
            passed += 1
        except Exception as e:
            print(f"✗ Test failed: {e}")
            failed += 1
            import traceback
            traceback.print_exc()
    
    print_section("Test Summary")
    print(f"Passed: {passed}/{len(tests)}")
    print(f"Failed: {failed}/{len(tests)}")
    
    if failed == 0:
        print("\n🎉 All tests passed!")
        return 0
    else:
        print(f"\n❌ {failed} test(s) failed")
        return 1

if __name__ == "__main__":
    # Give server time to start
    if len(sys.argv) > 1 and sys.argv[1] == "--wait":
        print("Waiting 5 seconds for server to start...")
        time.sleep(5)
    
    sys.exit(run_all_tests())
