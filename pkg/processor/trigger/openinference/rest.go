/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0

OpenInference Protocol v2 REST API implementation based on:
https://github.com/kserve/open-inference-protocol/blob/main/specification/protocol/inference_rest.md
*/

package openinference

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nuclio/nuclio-sdk-go"
)

// REST API request/response structures

type RESTInferenceRequest struct {
	ID         string                  `json:"id,omitempty"`
	Parameters map[string]any          `json:"parameters,omitempty"`
	Inputs     []RESTInferInputTensor  `json:"inputs"`
	Outputs    []RESTInferOutputTensor `json:"outputs,omitempty"`
}

type RESTInferenceResponse struct {
	ModelName    string                  `json:"model_name"`
	ModelVersion string                  `json:"model_version,omitempty"`
	ID           string                  `json:"id,omitempty"`
	Parameters   map[string]any          `json:"parameters,omitempty"`
	Outputs      []RESTInferOutputTensor `json:"outputs"`
}

type RESTInferInputTensor struct {
	Name       string         `json:"name"`
	Shape      []int64        `json:"shape"`
	Datatype   string         `json:"datatype"`
	Parameters map[string]any `json:"parameters,omitempty"`
	Data       any            `json:"data"`
}

type RESTInferOutputTensor struct {
	Name       string         `json:"name"`
	Shape      []int64        `json:"shape,omitempty"`
	Datatype   string         `json:"datatype,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
	Data       any            `json:"data,omitempty"`
}

type ServerMetadataResponse struct {
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Extensions []string `json:"extensions,omitempty"`
}

type ModelMetadataResponse struct {
	Name     string               `json:"name"`
	Versions []string             `json:"versions,omitempty"`
	Platform string               `json:"platform,omitempty"`
	Inputs   []RESTTensorMetadata `json:"inputs,omitempty"`
	Outputs  []RESTTensorMetadata `json:"outputs,omitempty"`
}

type RESTTensorMetadata struct {
	Name     string  `json:"name"`
	Datatype string  `json:"datatype"`
	Shape    []int64 `json:"shape"`
}

type ServerLiveResponse struct {
	Live bool `json:"live"`
}

type ServerReadyResponse struct {
	Ready bool `json:"ready"`
}

type ModelReadyResponse struct {
	Ready bool `json:"ready"`
}

// Register REST API handlers
func (oi *openInference) registerRESTHandlers(mux *http.ServeMux) {
	// Health endpoints
	mux.HandleFunc("/v2/health/live", oi.handleServerLive)
	mux.HandleFunc("/v2/health/ready", oi.handleServerReady)

	// Model endpoints
	mux.HandleFunc("/v2/models/", oi.handleModelEndpoints)

	// Metadata endpoints
	mux.HandleFunc("/v2", oi.handleServerMetadata)

	oi.Logger.InfoWith("REST handlers registered")
}

func (oi *openInference) handleServerLive(w http.ResponseWriter, r *http.Request) {
	response := ServerLiveResponse{Live: true}
	oi.writeJSONResponse(w, http.StatusOK, response)
}

func (oi *openInference) handleServerReady(w http.ResponseWriter, r *http.Request) {
	response := ServerReadyResponse{Ready: true}
	oi.writeJSONResponse(w, http.StatusOK, response)
}

func (oi *openInference) handleModelEndpoints(w http.ResponseWriter, r *http.Request) {
	// Parse URL to determine which endpoint
	// Format: /v2/models/{model_name}/[versions/{version}/][ready|infer|metadata]

	path := r.URL.Path

	// Simple routing based on path suffix
	if strings.HasSuffix(path, "/ready") {
		oi.handleModelReady(w, r)
	} else if strings.HasSuffix(path, "/infer") {
		oi.handleModelInfer(w, r)
	} else if strings.HasSuffix(path, "/models/"+oi.configuration.ModelName) || strings.HasSuffix(path, "/models/"+oi.configuration.ModelName+"/versions/"+oi.configuration.ModelVersion) {
		oi.handleModelMetadata(w, r)
	} else {
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func (oi *openInference) handleServerMetadata(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v2" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	response := ServerMetadataResponse{
		Name:       "digitalhub-serverless",
		Version:    "1.0.0",
		Extensions: []string{},
	}
	oi.writeJSONResponse(w, http.StatusOK, response)
}

func (oi *openInference) handleModelMetadata(w http.ResponseWriter, _ *http.Request) {
	inputs := make([]RESTTensorMetadata, len(oi.configuration.InputTensors))
	for i, tensor := range oi.configuration.InputTensors {
		inputs[i] = RESTTensorMetadata{
			Name:     tensor.Name,
			Datatype: tensor.DataType,
			Shape:    tensor.Shape,
		}
	}

	outputs := make([]RESTTensorMetadata, len(oi.configuration.OutputTensors))
	for i, tensor := range oi.configuration.OutputTensors {
		outputs[i] = RESTTensorMetadata{
			Name:     tensor.Name,
			Datatype: tensor.DataType,
			Shape:    tensor.Shape,
		}
	}

	response := ModelMetadataResponse{
		Name:     oi.configuration.ModelName,
		Versions: []string{oi.configuration.ModelVersion},
		Platform: oi.configuration.RuntimeConfiguration.Spec.Runtime,
		Inputs:   inputs,
		Outputs:  outputs,
	}
	oi.writeJSONResponse(w, http.StatusOK, response)
}

func (oi *openInference) handleModelReady(w http.ResponseWriter, _ *http.Request) {
	response := ModelReadyResponse{Ready: true}
	oi.writeJSONResponse(w, http.StatusOK, response)
}

func (oi *openInference) handleModelInfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		oi.Logger.WarnWith("Failed to read request body", "error", err)
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse inference request
	var inferRequest RESTInferenceRequest
	if err := json.Unmarshal(body, &inferRequest); err != nil {
		oi.Logger.WarnWith("Failed to parse inference request", "error", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Create nuclio event with the REST inference request as body
	event := &Event{
		body: body,
		headers: map[string]any{
			"X-Model-Name":    oi.configuration.ModelName,
			"X-Model-Version": oi.configuration.ModelVersion,
			"X-Request-ID":    inferRequest.ID,
		},
		timestamp:    time.Now(),
		modelName:    oi.configuration.ModelName,
		modelVersion: oi.configuration.ModelVersion,
		parameters:   inferRequest.Parameters,
	}

	// Submit to worker
	response, submitError, processError := oi.AllocateWorkerAndSubmitEvent(
		event,
		oi.Logger,
		10*time.Second,
	)

	if submitError != nil {
		oi.Logger.WarnWith("Failed to submit event", "error", submitError)
		http.Error(w, "Failed to process request", http.StatusInternalServerError)
		return
	}

	if processError != nil {
		oi.Logger.WarnWith("Failed to process event", "error", processError)
		http.Error(w, "Failed to process request", http.StatusInternalServerError)
		return
	}

	// Handle response
	switch typedResponse := response.(type) {
	case nuclio.Response:
		// Parse the response body as inference response
		var inferResponse RESTInferenceResponse
		if err := json.Unmarshal(typedResponse.Body, &inferResponse); err != nil {
			oi.Logger.WarnWith("Failed to parse function response", "error", err)
			http.Error(w, "Invalid function response", http.StatusInternalServerError)
			return
		}

		// Set model name and version if not present
		if inferResponse.ModelName == "" {
			inferResponse.ModelName = oi.configuration.ModelName
		}
		if inferResponse.ModelVersion == "" {
			inferResponse.ModelVersion = oi.configuration.ModelVersion
		}
		if inferResponse.ID == "" && inferRequest.ID != "" {
			inferResponse.ID = inferRequest.ID
		}

		oi.writeJSONResponse(w, typedResponse.StatusCode, inferResponse)

	default:
		// If response is not a nuclio.Response, convert it to JSON
		oi.writeJSONResponse(w, http.StatusOK, response)
	}
}

func (oi *openInference) writeJSONResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		oi.Logger.WarnWith("Failed to write JSON response", "error", err)
	}
}
