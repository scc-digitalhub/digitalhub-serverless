/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package openinference

import (
	"context"
	"testing"

	pb "github.com/scc-digitalhub/digitalhub-serverless/pkg/proto/inference/v2"
	"github.com/stretchr/testify/assert"
)

func TestGRPCServerLive(t *testing.T) {
	oi := createTestOpenInferenceTrigger(t)
	server := &grpcInferenceServer{trigger: oi}

	req := &pb.ServerLiveRequest{}
	resp, err := server.ServerLive(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Live)
}

func TestGRPCServerReady(t *testing.T) {
	oi := createTestOpenInferenceTrigger(t)
	server := &grpcInferenceServer{trigger: oi}

	req := &pb.ServerReadyRequest{}
	resp, err := server.ServerReady(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ready)
}

func TestGRPCModelReady(t *testing.T) {
	oi := createTestOpenInferenceTrigger(t)
	server := &grpcInferenceServer{trigger: oi}

	t.Run("CorrectModelName", func(t *testing.T) {
		req := &pb.ModelReadyRequest{
			Name:    "test-model",
			Version: "1.0",
		}
		resp, err := server.ModelReady(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.Ready)
	})

	t.Run("WrongModelName", func(t *testing.T) {
		req := &pb.ModelReadyRequest{
			Name:    "wrong-model",
			Version: "1.0",
		}
		resp, err := server.ModelReady(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.Ready)
	})
}

func TestGRPCServerMetadata(t *testing.T) {
	oi := createTestOpenInferenceTrigger(t)
	server := &grpcInferenceServer{trigger: oi}

	req := &pb.ServerMetadataRequest{}
	resp, err := server.ServerMetadata(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "digitalhub-serverless", resp.Name)
	assert.NotEmpty(t, resp.Version)
}

func TestGRPCModelMetadata(t *testing.T) {
	oi := createTestOpenInferenceTrigger(t)
	server := &grpcInferenceServer{trigger: oi}

	req := &pb.ModelMetadataRequest{
		Name:    "test-model",
		Version: "1.0",
	}
	resp, err := server.ModelMetadata(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-model", resp.Name)
	// Note: gRPC proto uses Versions (plural), not Version
	assert.Len(t, resp.Inputs, 1)
	assert.Equal(t, "input", resp.Inputs[0].Name)
	assert.Equal(t, "FP32", resp.Inputs[0].Datatype)
	assert.Equal(t, []int64{1, 3}, resp.Inputs[0].Shape)
	assert.Len(t, resp.Outputs, 1)
	assert.Equal(t, "output", resp.Outputs[0].Name)
}

// Note: The following tests are removed because they test private functions.
// The actual conversion logic is tested through integration tests.
