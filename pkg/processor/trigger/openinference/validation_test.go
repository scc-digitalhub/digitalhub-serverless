package openinference

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net/http"
	"testing"

	pb "github.com/scc-digitalhub/digitalhub-serverless/pkg/proto/inference/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidationErrorMapping(t *testing.T) {
	invalid := fmt.Errorf("wrapped: %w", &InvalidInputError{Err: fmt.Errorf("wrong shape")})
	code, message := inferenceHTTPError(invalid)
	require.Equal(t, http.StatusBadRequest, code)
	require.Contains(t, message, "wrong shape")
	require.Equal(t, codes.InvalidArgument, status.Code(inferenceGRPCError(invalid)))
	internal := fmt.Errorf("worker failed")
	code, _ = inferenceHTTPError(internal)
	require.Equal(t, http.StatusInternalServerError, code)
	require.Same(t, internal, inferenceGRPCError(internal))
}

func TestUnknownModelRejected(t *testing.T) {
	server := &grpcInferenceServer{trigger: createTestOpenInferenceTrigger(t)}
	for _, name := range []string{"missing-model", ""} {
		_, err := server.ModelMetadata(context.Background(), &pb.ModelMetadataRequest{Name: name})
		require.Equal(t, codes.NotFound, status.Code(err))
		_, err = server.ModelInfer(context.Background(), &pb.ModelInferRequest{ModelName: name})
		require.Equal(t, codes.NotFound, status.Code(err))
	}
}

func TestMalformedGRPCInputs(t *testing.T) {
	server := &grpcInferenceServer{trigger: createTestOpenInferenceTrigger(t)}
	for _, tc := range []struct {
		name   string
		inputs []*pb.ModelInferRequest_InferInputTensor
		raw    [][]byte
	}{
		{"missing", []*pb.ModelInferRequest_InferInputTensor{{Datatype: "FP32"}}, nil},
		{"unaligned raw", []*pb.ModelInferRequest_InferInputTensor{{Datatype: "FP32"}}, [][]byte{{1}}},
		{"unsupported", []*pb.ModelInferRequest_InferInputTensor{{Datatype: "FP16", Contents: &pb.InferTensorContents{}}}, nil},
		{"mixed", []*pb.ModelInferRequest_InferInputTensor{{Datatype: "FP32", Contents: &pb.InferTensorContents{Fp32Contents: []float32{1}}}}, [][]byte{{0, 0, 128, 63}}},
		{"extra raw", []*pb.ModelInferRequest_InferInputTensor{{Datatype: "FP32"}}, [][]byte{{0, 0, 0, 0}, {0, 0, 0, 0}}},
		{"missing raw", []*pb.ModelInferRequest_InferInputTensor{{Datatype: "FP32"}, {Datatype: "FP32"}}, [][]byte{{0, 0, 0, 0}}},
		{"wrong typed field", []*pb.ModelInferRequest_InferInputTensor{{Datatype: "FP32", Shape: []int64{1}, Contents: &pb.InferTensorContents{IntContents: []int32{1}}}}, nil},
		{"extra typed field", []*pb.ModelInferRequest_InferInputTensor{{Datatype: "FP32", Shape: []int64{1}, Contents: &pb.InferTensorContents{Fp32Contents: []float32{1}, IntContents: []int32{2}}}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := server.ModelInfer(context.Background(), &pb.ModelInferRequest{ModelName: "test-model", Inputs: tc.inputs, RawInputContents: tc.raw})
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestEmptyTypedTensorContents(t *testing.T) {
	server := &grpcInferenceServer{}
	for _, datatype := range []string{"BOOL", "INT8", "INT16", "INT32", "INT64", "UINT8", "UINT16", "UINT32", "UINT64", "FP32", "FP64", "BYTES"} {
		t.Run(datatype, func(t *testing.T) {
			request, err := server.convertGRPCToRESTRequest(&pb.ModelInferRequest{
				Inputs: []*pb.ModelInferRequest_InferInputTensor{{
					Name: "empty", Datatype: datatype, Shape: []int64{0}, Contents: &pb.InferTensorContents{},
				}},
			})
			require.NoError(t, err)
			body, err := Serialize(request)
			require.NoError(t, err)
			var decoded RESTInferenceRequest
			require.NoError(t, decodeJSON(body, &decoded))
			// Check the event body the runtime receives, where null would fail
			// numeric tensor validation even though its declared size is zero.
			require.Equal(t, []any{}, decoded.Inputs[0].Data)
		})
	}
}

func TestNonFiniteGRPCInputs(t *testing.T) {
	// This trigger has no worker: invalid values must fail before allocation.
	server := &grpcInferenceServer{trigger: createTestOpenInferenceTrigger(t)}
	for _, datatype := range []string{"FP32", "FP64"} {
		for _, representation := range []string{"typed", "raw"} {
			for _, value := range []struct {
				name string
				data float64
			}{
				{"NaN", math.NaN()}, {"positive infinity", math.Inf(1)}, {"negative infinity", math.Inf(-1)},
			} {
				t.Run(datatype+"/"+representation+"/"+value.name, func(t *testing.T) {
					input := &pb.ModelInferRequest_InferInputTensor{Name: "x", Datatype: datatype, Shape: []int64{1}}
					request := &pb.ModelInferRequest{ModelName: "test-model", Inputs: []*pb.ModelInferRequest_InferInputTensor{input}}
					if representation == "typed" {
						input.Contents = &pb.InferTensorContents{}
						if datatype == "FP32" {
							input.Contents.Fp32Contents = []float32{float32(value.data)}
						} else {
							input.Contents.Fp64Contents = []float64{value.data}
						}
					} else if datatype == "FP32" {
						request.RawInputContents = [][]byte{binary.LittleEndian.AppendUint32(nil, math.Float32bits(float32(value.data)))}
					} else {
						request.RawInputContents = [][]byte{binary.LittleEndian.AppendUint64(nil, math.Float64bits(value.data))}
					}
					_, err := server.ModelInfer(context.Background(), request)
					require.Equal(t, codes.InvalidArgument, status.Code(err))
				})
			}
		}
	}
}
