/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0

OpenInference Protocol v2 gRPC implementation based on:
https://github.com/kserve/open-inference-protocol/blob/main/specification/protocol/inference_grpc.md
*/

package openinference

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nuclio/nuclio-sdk-go"
	pb "github.com/scc-digitalhub/digitalhub-serverless/pkg/proto/inference/v2"
	"google.golang.org/grpc"
)

// grpcInferenceServer implements the GRPCInferenceService
type grpcInferenceServer struct {
	pb.UnimplementedGRPCInferenceServiceServer
	trigger *openInference
}

// Register gRPC handlers
func (oi *openInference) registerGRPCHandlers(server *grpc.Server) {
	grpcServer := &grpcInferenceServer{
		trigger: oi,
	}
	pb.RegisterGRPCInferenceServiceServer(server, grpcServer)
	oi.Logger.InfoWith("gRPC handlers registered")
}

// ServerLive - Check liveness of the inference server
func (s *grpcInferenceServer) ServerLive(ctx context.Context, req *pb.ServerLiveRequest) (*pb.ServerLiveResponse, error) {
	return &pb.ServerLiveResponse{
		Live: true,
	}, nil
}

// ServerReady - Check readiness of the inference server
func (s *grpcInferenceServer) ServerReady(ctx context.Context, req *pb.ServerReadyRequest) (*pb.ServerReadyResponse, error) {
	return &pb.ServerReadyResponse{
		Ready: true,
	}, nil
}

// ModelReady - Check if a model is ready
func (s *grpcInferenceServer) ModelReady(ctx context.Context, req *pb.ModelReadyRequest) (*pb.ModelReadyResponse, error) {
	// Check if the requested model matches our configuration
	ready := req.Name == s.trigger.configuration.ModelName
	if req.Version != "" {
		ready = ready && req.Version == s.trigger.configuration.ModelVersion
	}

	return &pb.ModelReadyResponse{
		Ready: ready,
	}, nil
}

// ServerMetadata - Get server metadata
func (s *grpcInferenceServer) ServerMetadata(ctx context.Context, req *pb.ServerMetadataRequest) (*pb.ServerMetadataResponse, error) {
	return &pb.ServerMetadataResponse{
		Name:       "digitalhub-serverless",
		Version:    "1.0.0",
		Extensions: []string{},
	}, nil
}

// ModelMetadata - Get model metadata
func (s *grpcInferenceServer) ModelMetadata(ctx context.Context, req *pb.ModelMetadataRequest) (*pb.ModelMetadataResponse, error) {
	// Convert input tensor definitions to metadata
	inputs := make([]*pb.TensorMetadata, len(s.trigger.configuration.InputTensors))
	for i, tensor := range s.trigger.configuration.InputTensors {
		inputs[i] = &pb.TensorMetadata{
			Name:     tensor.Name,
			Datatype: tensor.DataType,
			Shape:    tensor.Shape,
		}
	}

	// Convert output tensor definitions to metadata
	outputs := make([]*pb.TensorMetadata, len(s.trigger.configuration.OutputTensors))
	for i, tensor := range s.trigger.configuration.OutputTensors {
		outputs[i] = &pb.TensorMetadata{
			Name:     tensor.Name,
			Datatype: tensor.DataType,
			Shape:    tensor.Shape,
		}
	}

	return &pb.ModelMetadataResponse{
		Name:     s.trigger.configuration.ModelName,
		Versions: []string{s.trigger.configuration.ModelVersion},
		Platform: s.trigger.configuration.RuntimeConfiguration.Spec.Runtime,
		Inputs:   inputs,
		Outputs:  outputs,
	}, nil
}

// ModelInfer - Perform inference using a specific model
func (s *grpcInferenceServer) ModelInfer(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
	// Convert gRPC request to REST format for processing
	restRequest := s.convertGRPCToRESTRequest(req)

	// Marshal to JSON for the event body
	body, err := json.Marshal(restRequest)
	if err != nil {
		s.trigger.Logger.WarnWith("Failed to marshal request", "error", err)
		return nil, err
	}

	// Create nuclio event
	event := &Event{
		body: body,
		headers: map[string]any{
			"X-Model-Name":    s.trigger.configuration.ModelName,
			"X-Model-Version": s.trigger.configuration.ModelVersion,
			"X-Request-ID":    req.Id,
			"X-Protocol":      "grpc",
		},
		timestamp:    time.Now(),
		modelName:    req.ModelName,
		modelVersion: s.trigger.configuration.ModelVersion,
		parameters:   convertParametersToMap(req.Parameters),
	}

	// Submit to worker
	response, submitError, processError := s.trigger.AllocateWorkerAndSubmitEvent(
		event,
		s.trigger.Logger,
		10*time.Second,
	)

	if submitError != nil {
		s.trigger.Logger.WarnWith("Failed to submit event", "error", submitError)
		return nil, submitError
	}

	if processError != nil {
		s.trigger.Logger.WarnWith("Failed to process event", "error", processError)
		return nil, processError
	}

	// Convert response to gRPC format
	switch typedResponse := response.(type) {
	case nuclio.Response:
		// Parse the response body
		var restResponse RESTInferenceResponse
		if err := json.Unmarshal(typedResponse.Body, &restResponse); err != nil {
			s.trigger.Logger.WarnWith("Failed to parse function response", "error", err)
			return nil, err
		}

		return s.convertRESTToGRPCResponse(&restResponse, req.Id), nil

	default:
		s.trigger.Logger.WarnWith("Unexpected response type", "type", typedResponse)
		return nil, nil
	}
}

func convertParametersToMap(params map[string]*pb.InferParameter) map[string]any {
	result := make(map[string]any)
	for key, param := range params {
		switch p := param.ParameterChoice.(type) {
		case *pb.InferParameter_BoolParam:
			result[key] = p.BoolParam
		case *pb.InferParameter_Int64Param:
			result[key] = p.Int64Param
		case *pb.InferParameter_StringParam:
			result[key] = p.StringParam
		}
	}
	return result
}

// Helper to convert gRPC request to REST request
func (s *grpcInferenceServer) convertGRPCToRESTRequest(req *pb.ModelInferRequest) *RESTInferenceRequest {
	restReq := &RESTInferenceRequest{
		ID:         req.Id,
		Parameters: make(map[string]any),
		Inputs:     make([]RESTInferInputTensor, len(req.Inputs)),
		Outputs:    make([]RESTInferOutputTensor, len(req.Outputs)),
	}

	// Convert parameters
	for key, param := range req.Parameters {
		switch p := param.ParameterChoice.(type) {
		case *pb.InferParameter_BoolParam:
			restReq.Parameters[key] = p.BoolParam
		case *pb.InferParameter_Int64Param:
			restReq.Parameters[key] = p.Int64Param
		case *pb.InferParameter_StringParam:
			restReq.Parameters[key] = p.StringParam
		}
	}

	// Convert input tensors
	for i, input := range req.Inputs {
		restReq.Inputs[i] = RESTInferInputTensor{
			Name:       input.Name,
			Shape:      input.Shape,
			Datatype:   input.Datatype,
			Parameters: make(map[string]any),
			Data:       s.convertTensorContents(input.Contents, input.Datatype),
		}

		for key, param := range input.Parameters {
			switch p := param.ParameterChoice.(type) {
			case *pb.InferParameter_BoolParam:
				restReq.Inputs[i].Parameters[key] = p.BoolParam
			case *pb.InferParameter_Int64Param:
				restReq.Inputs[i].Parameters[key] = p.Int64Param
			case *pb.InferParameter_StringParam:
				restReq.Inputs[i].Parameters[key] = p.StringParam
			}
		}
	}

	// Convert requested outputs
	for i, output := range req.Outputs {
		restReq.Outputs[i] = RESTInferOutputTensor{
			Name:       output.Name,
			Parameters: make(map[string]any),
		}

		for key, param := range output.Parameters {
			switch p := param.ParameterChoice.(type) {
			case *pb.InferParameter_BoolParam:
				restReq.Outputs[i].Parameters[key] = p.BoolParam
			case *pb.InferParameter_Int64Param:
				restReq.Outputs[i].Parameters[key] = p.Int64Param
			case *pb.InferParameter_StringParam:
				restReq.Outputs[i].Parameters[key] = p.StringParam
			}
		}
	}

	return restReq
}

// Helper to convert REST response to gRPC response
func (s *grpcInferenceServer) convertRESTToGRPCResponse(resp *RESTInferenceResponse, requestID string) *pb.ModelInferResponse {
	grpcResp := &pb.ModelInferResponse{
		ModelName:    resp.ModelName,
		ModelVersion: resp.ModelVersion,
		Id:           resp.ID,
		Parameters:   make(map[string]*pb.InferParameter),
		Outputs:      make([]*pb.ModelInferResponse_InferOutputTensor, len(resp.Outputs)),
	}

	if grpcResp.Id == "" {
		grpcResp.Id = requestID
	}

	// Convert parameters
	for key, value := range resp.Parameters {
		switch v := value.(type) {
		case bool:
			grpcResp.Parameters[key] = &pb.InferParameter{
				ParameterChoice: &pb.InferParameter_BoolParam{BoolParam: v},
			}
		case int64:
			grpcResp.Parameters[key] = &pb.InferParameter{
				ParameterChoice: &pb.InferParameter_Int64Param{Int64Param: v},
			}
		case float64:
			grpcResp.Parameters[key] = &pb.InferParameter{
				ParameterChoice: &pb.InferParameter_Int64Param{Int64Param: int64(v)},
			}
		case string:
			grpcResp.Parameters[key] = &pb.InferParameter{
				ParameterChoice: &pb.InferParameter_StringParam{StringParam: v},
			}
		}
	}

	// Convert output tensors
	for i, output := range resp.Outputs {
		tensor := &pb.ModelInferResponse_InferOutputTensor{
			Name:       output.Name,
			Datatype:   output.Datatype,
			Shape:      output.Shape,
			Parameters: make(map[string]*pb.InferParameter),
			Contents:   s.convertDataToTensorContents(output.Data, output.Datatype),
		}

		for key, value := range output.Parameters {
			switch v := value.(type) {
			case bool:
				tensor.Parameters[key] = &pb.InferParameter{
					ParameterChoice: &pb.InferParameter_BoolParam{BoolParam: v},
				}
			case int64:
				tensor.Parameters[key] = &pb.InferParameter{
					ParameterChoice: &pb.InferParameter_Int64Param{Int64Param: v},
				}
			case float64:
				tensor.Parameters[key] = &pb.InferParameter{
					ParameterChoice: &pb.InferParameter_Int64Param{Int64Param: int64(v)},
				}
			case string:
				tensor.Parameters[key] = &pb.InferParameter{
					ParameterChoice: &pb.InferParameter_StringParam{StringParam: v},
				}
			}
		}

		grpcResp.Outputs[i] = tensor
	}

	return grpcResp
}

// Helper to convert tensor contents from protobuf to generic data
func (s *grpcInferenceServer) convertTensorContents(contents *pb.InferTensorContents, datatype string) any {
	if contents == nil {
		return nil
	}

	switch datatype {
	case "BOOL":
		return contents.BoolContents
	case "INT8", "INT16", "INT32":
		return contents.IntContents
	case "INT64":
		return contents.Int64Contents
	case "UINT8", "UINT16", "UINT32":
		return contents.UintContents
	case "UINT64":
		return contents.Uint64Contents
	case "FP32":
		return contents.Fp32Contents
	case "FP64":
		return contents.Fp64Contents
	case "BYTES":
		return contents.BytesContents
	default:
		return nil
	}
}

// Helper to convert generic data to tensor contents protobuf
func (s *grpcInferenceServer) convertDataToTensorContents(data any, datatype string) *pb.InferTensorContents {
	contents := &pb.InferTensorContents{}

	switch datatype {
	case "BOOL":
		if bools, ok := data.([]bool); ok {
			contents.BoolContents = bools
		}
	case "INT8", "INT16", "INT32":
		if ints, ok := data.([]int32); ok {
			contents.IntContents = ints
		} else if floats, ok := data.([]float64); ok {
			// Convert from float64 (JSON default) to int32
			ints := make([]int32, len(floats))
			for i, f := range floats {
				ints[i] = int32(f)
			}
			contents.IntContents = ints
		} else if arr, ok := data.([]any); ok {
			// Handle JSON array
			ints := make([]int32, len(arr))
			for i, v := range arr {
				if f, ok := v.(float64); ok {
					ints[i] = int32(f)
				}
			}
			contents.IntContents = ints
		}
	case "INT64":
		if ints, ok := data.([]int64); ok {
			contents.Int64Contents = ints
		} else if floats, ok := data.([]float64); ok {
			// Convert from float64 (JSON default) to int64
			ints := make([]int64, len(floats))
			for i, f := range floats {
				ints[i] = int64(f)
			}
			contents.Int64Contents = ints
		} else if arr, ok := data.([]any); ok {
			// Handle JSON array
			ints := make([]int64, len(arr))
			for i, v := range arr {
				if f, ok := v.(float64); ok {
					ints[i] = int64(f)
				}
			}
			contents.Int64Contents = ints
		}
	case "UINT8", "UINT16", "UINT32":
		if uints, ok := data.([]uint32); ok {
			contents.UintContents = uints
		} else if arr, ok := data.([]any); ok {
			// Handle JSON array
			uints := make([]uint32, len(arr))
			for i, v := range arr {
				if f, ok := v.(float64); ok {
					uints[i] = uint32(f)
				}
			}
			contents.UintContents = uints
		}
	case "UINT64":
		if uints, ok := data.([]uint64); ok {
			contents.Uint64Contents = uints
		} else if arr, ok := data.([]any); ok {
			// Handle JSON array
			uints := make([]uint64, len(arr))
			for i, v := range arr {
				if f, ok := v.(float64); ok {
					uints[i] = uint64(f)
				}
			}
			contents.Uint64Contents = uints
		}
	case "FP32":
		if floats, ok := data.([]float32); ok {
			contents.Fp32Contents = floats
		} else if floats64, ok := data.([]float64); ok {
			// Convert from float64 (JSON default) to float32
			floats := make([]float32, len(floats64))
			for i, f := range floats64 {
				floats[i] = float32(f)
			}
			contents.Fp32Contents = floats
		} else if arr, ok := data.([]any); ok {
			// Handle JSON array
			floats := make([]float32, len(arr))
			for i, v := range arr {
				if f, ok := v.(float64); ok {
					floats[i] = float32(f)
				}
			}
			contents.Fp32Contents = floats
		}
	case "FP64":
		if floats, ok := data.([]float64); ok {
			contents.Fp64Contents = floats
		} else if arr, ok := data.([]any); ok {
			// Handle JSON array
			floats := make([]float64, len(arr))
			for i, v := range arr {
				if f, ok := v.(float64); ok {
					floats[i] = f
				}
			}
			contents.Fp64Contents = floats
		}
	case "BYTES":
		if bytes, ok := data.([][]byte); ok {
			contents.BytesContents = bytes
		}
	}

	return contents
}
