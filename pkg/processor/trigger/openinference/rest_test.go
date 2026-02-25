/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package openinference

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/assert"
)

func createTestOpenInferenceTrigger(t *testing.T) *openInference {
	testLogger, err := nucliozap.NewNuclioZapTest("test")
	assert.NoError(t, err)

	// Create runtime configuration with proper Spec.Runtime field
	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Meta: functionconfig.Meta{
					Name:      "test-function",
					Namespace: "default",
				},
				Spec: functionconfig.Spec{
					Runtime: "python:3.11",
					Handler: "test_handler:handler",
				},
			},
			PlatformConfig: &platformconfig.Config{
				Kind: "local",
			},
		},
	}

	triggerConfig := &functionconfig.Trigger{
		Kind: "openinference",
		Attributes: map[string]interface{}{
			"model_name":    "test-model",
			"model_version": "1.0",
			"rest_port":     8080,
			"grpc_port":     9000,
			"enable_rest":   true,
			"enable_grpc":   true,
			"input_tensors": []map[string]interface{}{
				{
					"name":     "input",
					"datatype": "FP32",
					"shape":    []int64{1, 3},
				},
			},
			"output_tensors": []map[string]interface{}{
				{
					"name":     "output",
					"datatype": "FP32",
					"shape":    []int64{1, 1},
				},
			},
		},
	}

	// Use NewConfiguration to properly create configuration with runtime config
	config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
	assert.NoError(t, err)

	oi := &openInference{
		configuration: config,
	}
	oi.Logger = testLogger

	return oi
}

func TestRESTHandleServerLive(t *testing.T) {
	oi := createTestOpenInferenceTrigger(t)

	req := httptest.NewRequest("GET", "/v2/health/live", nil)
	w := httptest.NewRecorder()

	oi.handleServerLive(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ServerLiveResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Live)
}

func TestRESTHandleServerReady(t *testing.T) {
	oi := createTestOpenInferenceTrigger(t)

	req := httptest.NewRequest("GET", "/v2/health/ready", nil)
	w := httptest.NewRecorder()

	oi.handleServerReady(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ServerReadyResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Ready)
}

func TestRESTHandleModelMetadata(t *testing.T) {
	oi := createTestOpenInferenceTrigger(t)

	req := httptest.NewRequest("GET", "/v2/models/test-model", nil)
	w := httptest.NewRecorder()

	oi.handleModelMetadata(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ModelMetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test-model", response.Name)
	// Note: gRPC proto uses Versions (plural)
	assert.Len(t, response.Inputs, 1)
	assert.Equal(t, "input", response.Inputs[0].Name)
	assert.Equal(t, "FP32", response.Inputs[0].Datatype)
	assert.Equal(t, []int64{1, 3}, response.Inputs[0].Shape)
	assert.Len(t, response.Outputs, 1)
	assert.Equal(t, "output", response.Outputs[0].Name)
}

func TestRESTJSONSerialization(t *testing.T) {
	t.Run("InferenceInputTensor", func(t *testing.T) {
		tensor := RESTInferInputTensor{
			Name:     "input",
			Datatype: "FP32",
			Shape:    []int64{1, 3},
			Data:     []interface{}{1.0, 2.0, 3.0},
		}

		body, err := json.Marshal(tensor)
		assert.NoError(t, err)

		var decoded RESTInferInputTensor
		err = json.Unmarshal(body, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, "input", decoded.Name)
		assert.Equal(t, []int64{1, 3}, decoded.Shape)
		assert.Len(t, decoded.Data, 3)
	})

	t.Run("InferenceOutputTensor", func(t *testing.T) {
		tensor := RESTInferOutputTensor{
			Name:     "output",
			Datatype: "FP32",
			Shape:    []int64{1, 1},
			Data:     []interface{}{0.95},
		}

		body, err := json.Marshal(tensor)
		assert.NoError(t, err)

		var decoded RESTInferOutputTensor
		err = json.Unmarshal(body, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, "output", decoded.Name)
	})
}
