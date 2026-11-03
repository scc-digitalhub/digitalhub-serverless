package openinference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	nuclioerrors "github.com/nuclio/errors"
	"github.com/nuclio/logger"
	nuclio "github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	pb "github.com/scc-digitalhub/digitalhub-serverless/pkg/proto/inference/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recoveringAllocator struct {
	worker.Allocator
	nextError error
}

func (a *recoveringAllocator) Allocate(timeout time.Duration) (*worker.Worker, error) {
	if err := a.nextError; err != nil {
		a.nextError = nil
		return nil, err
	}
	return a.Allocator.Allocate(timeout)
}

type recoveryRuntime struct {
	runtime.Runtime
	calls int
}

func (r *recoveryRuntime) ProcessEvent(nuclio.Event, logger.Logger) (interface{}, error) {
	r.calls++
	return nuclio.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"outputs":[{"name":"output","datatype":"FP32","shape":[1],"data":[42]}]}`),
	}, nil
}

func TestWorkerAllocationErrorsAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		httpCode int
		grpcCode codes.Code
	}{
		{"exhausted", worker.ErrNoAvailableWorkers, http.StatusServiceUnavailable, codes.ResourceExhausted},
		{"standard wrapped", fmt.Errorf("allocator: %w", worker.ErrNoAvailableWorkers), http.StatusServiceUnavailable, codes.ResourceExhausted},
		{"nuclio wrapped", nuclioerrors.Wrap(worker.ErrNoAvailableWorkers, "allocator"), http.StatusServiceUnavailable, codes.ResourceExhausted},
		{"same message different error", errors.New(worker.ErrNoAvailableWorkers.Error()), http.StatusInternalServerError, codes.Unknown},
		{"unrelated failure", errors.New("allocator failed"), http.StatusInternalServerError, codes.Unknown},
		{"explicit internal status", status.Error(codes.Internal, "allocator failed"), http.StatusInternalServerError, codes.Internal},
		{"terminated workers", worker.ErrAllWorkersAreTerminated, http.StatusInternalServerError, codes.Unknown},
	} {
		for _, protocol := range []string{"REST", "gRPC"} {
			t.Run(tc.name+"/"+protocol, func(t *testing.T) {
				oi := createTestOpenInferenceTrigger(t)
				r := &recoveryRuntime{}
				w, err := worker.NewWorker(oi.Logger, 0, r)
				require.NoError(t, err)
				allocator, err := worker.NewSingletonWorkerAllocator(oi.Logger, w)
				require.NoError(t, err)
				oi.WorkerAllocator = &recoveringAllocator{Allocator: allocator, nextError: tc.err}

				if protocol == "REST" {
					mux := http.NewServeMux()
					oi.registerRESTHandlers(mux)
					infer := func() *httptest.ResponseRecorder {
						request := httptest.NewRequest(http.MethodPost, "/v2/models/test-model/infer", strings.NewReader(
							`{"inputs":[{"name":"input","datatype":"FP32","shape":[1],"data":[1]}]}`))
						response := httptest.NewRecorder()
						mux.ServeHTTP(response, request)
						return response
					}
					require.Equal(t, tc.httpCode, infer().Code)
					require.Zero(t, r.calls, "an allocation failure must not invoke the runtime")
					response := infer()
					require.Equal(t, http.StatusOK, response.Code)
					var decoded RESTInferenceResponse
					require.NoError(t, json.Unmarshal(response.Body.Bytes(), &decoded))
					require.Len(t, decoded.Outputs, 1)
					require.Equal(t, []any{float64(42)}, decoded.Outputs[0].Data)
				} else {
					server := &grpcInferenceServer{trigger: oi}
					request := &pb.ModelInferRequest{
						ModelName: "test-model",
						Inputs: []*pb.ModelInferRequest_InferInputTensor{{
							Name: "input", Datatype: "FP32", Shape: []int64{1},
							Contents: &pb.InferTensorContents{Fp32Contents: []float32{1}},
						}},
					}
					_, err = server.ModelInfer(context.Background(), request)
					require.Equal(t, tc.grpcCode, status.Code(err))
					require.Zero(t, r.calls, "an allocation failure must not invoke the runtime")
					response, err := server.ModelInfer(context.Background(), request)
					require.NoError(t, err)
					require.Len(t, response.Outputs, 1)
					require.Equal(t, []float32{42}, response.Outputs[0].Contents.Fp32Contents)
				}
				require.Equal(t, 1, r.calls, "the next request must recover after worker availability returns")
			})
		}
	}
}
