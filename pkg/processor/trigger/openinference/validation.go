package openinference

import (
	"errors"
	"net/http"

	"github.com/nuclio/nuclio/pkg/processor/worker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// InvalidInputError lets a runtime distinguish invalid tensor requests from execution failures.
type InvalidInputError struct {
	Err error
}

func (e *InvalidInputError) Error() string { return e.Err.Error() }
func (e *InvalidInputError) Unwrap() error { return e.Err }

func inferenceHTTPError(err error) (int, string) {
	var invalid *InvalidInputError
	if errors.As(err, &invalid) {
		return http.StatusBadRequest, invalid.Error()
	}
	if errors.Is(err, worker.ErrNoAvailableWorkers) {
		return http.StatusServiceUnavailable, "No available workers"
	}
	return http.StatusInternalServerError, "Failed to process request"
}

func inferenceGRPCError(err error) error {
	var invalid *InvalidInputError
	if errors.As(err, &invalid) {
		return status.Error(codes.InvalidArgument, invalid.Error())
	}
	if errors.Is(err, worker.ErrNoAvailableWorkers) {
		return status.Error(codes.ResourceExhausted, "No available workers")
	}
	return err
}
