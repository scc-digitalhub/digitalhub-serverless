package job

import (
	"testing"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLogger struct {
	logger.Logger
}

func (l *mockLogger) GetChild(string) logger.Logger { return l }

func TestJobFactory(t *testing.T) {
	tests := []struct {
		name                 string
		triggerConfig        *functionconfig.Trigger
		runtimeConfig        *runtime.Configuration
		workerAllocators     *worker.AllocatorSyncMap
		expectedError        bool
		expectedErrorMessage string
	}{
		{
			name: "valid configuration",
			triggerConfig: &functionconfig.Trigger{
				Kind: "job",
				Attributes: map[string]interface{}{
					"body": "test body",
				},
			},
			runtimeConfig:    &runtime.Configuration{},
			workerAllocators: worker.NewAllocatorSyncMap(),
			expectedError:    false,
		},
		{
			name: "invalid configuration",
			triggerConfig: &functionconfig.Trigger{
				Kind: "job",
				Attributes: map[string]interface{}{
					"invalid": "value",
				},
			},
			runtimeConfig:        &runtime.Configuration{},
			workerAllocators:     worker.NewAllocatorSyncMap(),
			expectedError:        true,
			expectedErrorMessage: "Failed to create trigger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create factory
			f := &factory{}

			// Create mock logger
			mockLogger := &mockLogger{}

			// Create restart trigger channel
			restartTriggerChan := make(chan trigger.Trigger, 1)

			// Create trigger
			triggerInstance, err := f.Create(mockLogger,
				"test-job",
				tt.triggerConfig,
				tt.runtimeConfig,
				tt.workerAllocators,
				restartTriggerChan)

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorMessage)
			} else {
				require.NoError(t, err)
				require.NotNil(t, triggerInstance)
				assert.IsType(t, &job{}, triggerInstance)
			}
		})
	}
}
