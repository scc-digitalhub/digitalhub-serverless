/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package job

import (
	"errors"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/require"
)

var ErrInitializationFailed = errors.New("trigger initialization failed")

// TestTriggerCreation tests creating a job trigger with various configurations
func TestTriggerCreation(t *testing.T) {
	tests := []struct {
		name          string
		triggerConfig *functionconfig.Trigger
		runtimeConfig *runtime.Configuration
		wantErr       bool
	}{
		{
			name: "valid configuration",
			triggerConfig: &functionconfig.Trigger{
				Kind:                "job",
				Name:                "test-job",
				WorkerAllocatorName: "mock-allocator",
				Attributes: map[string]interface{}{
					"schedule":   "*/5 * * * *",
					"numWorkers": 1,
					"body":       []byte("test body"),
				},
			},
			runtimeConfig: &runtime.Configuration{
				Configuration: &processor.Configuration{
					Config: functionconfig.Config{
						Spec: functionconfig.Spec{
							Runtime: "python",
							Handler: "test_handler:handler",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger
			logger, err := nucliozap.NewNuclioZapTest("test")
			require.NoError(t, err)

			// Create a new factory
			f := &factory{}

			// Create empty worker allocator map
			namedWorkerAllocators := worker.NewAllocatorSyncMap()
			allocator, _ := newMockWorkerAllocator()
			namedWorkerAllocators.Store("mock-allocator", allocator)

			// Create trigger instance
			triggerInstance, err := f.Create(logger, "test-job", tt.triggerConfig, tt.runtimeConfig, namedWorkerAllocators, nil)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, triggerInstance)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, triggerInstance)

			// Verify it's a job trigger
			_, ok := triggerInstance.(*job)
			require.True(t, ok)
		})
	}
}
