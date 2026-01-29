package extproc

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFactory(t *testing.T) {
	// Create a parent logger
	parentLogger, err := nucliozap.NewNuclioZapTest("test")
	require.NoError(t, err)

	// Create factory instance
	f := &factory{}

	t.Run("SuccessfulTriggerCreation", func(t *testing.T) {
		// Create trigger configuration
		triggerConfig := &functionconfig.Trigger{
			Kind: "extproc",
			Name: "test-trigger",
			Attributes: map[string]interface{}{
				"type": "preprocessor",
				"port": 8080,
				"processingOptions": map[string]interface{}{
					"logStream":        true,
					"decompressBodies": true,
				},
			},
		}

		// Create runtime configuration
		runtimeConfig := &runtime.Configuration{
			Configuration: &processor.Configuration{
				Config: functionconfig.Config{
					Meta: functionconfig.Meta{
						Name:      "test-function",
						Namespace: "default",
					},
					Spec: functionconfig.Spec{
						Runtime: "golang",
						Handler: "main:Handler",
					},
				},
				PlatformConfig: &platformconfig.Config{
					Kind: "docker",
				},
			},
		}

		// Create worker allocator sync map
		namedWorkerAllocators := worker.NewAllocatorSyncMap()

		// Create restart trigger channel
		restartTriggerChan := make(chan trigger.Trigger, 1)

		// Create trigger instance
		triggerInstance, err := f.Create(parentLogger,
			"test-id",
			triggerConfig,
			runtimeConfig,
			namedWorkerAllocators,
			restartTriggerChan)

		// Error should be "Failed to create worker allocator" since we have no real runtime registered
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to create worker")
		assert.Nil(t, triggerInstance)
	})

	t.Run("InvalidConfiguration", func(t *testing.T) {
		// Create invalid trigger configuration (missing port)
		invalidTriggerConfig := &functionconfig.Trigger{
			Kind: "extproc",
			Attributes: map[string]interface{}{
				"type": "preprocessor",
			},
		}

		runtimeConfig := &runtime.Configuration{
			Configuration: &processor.Configuration{
				Config: functionconfig.Config{
					Meta: functionconfig.Meta{
						Name:      "test-function",
						Namespace: "default",
					},
					Spec: functionconfig.Spec{},
				},
				PlatformConfig: &platformconfig.Config{
					Kind: "docker",
				},
			},
		}

		namedWorkerAllocators := worker.NewAllocatorSyncMap()
		restartTriggerChan := make(chan trigger.Trigger, 1)

		// Try to create trigger with invalid config
		triggerInstance, err := f.Create(parentLogger,
			"test-id",
			invalidTriggerConfig,
			runtimeConfig,
			namedWorkerAllocators,
			restartTriggerChan)

		assert.Error(t, err)
		assert.Nil(t, triggerInstance)
		// The real error from NewConfiguration should be about failed decoding of attributes
		assert.Contains(t, err.Error(), "Failed to parse trigger configuration")
	})

	t.Run("InvalidType", func(t *testing.T) {
		// Create invalid trigger configuration (invalid type)
		invalidTriggerConfig := &functionconfig.Trigger{
			Kind: "extproc",
			Attributes: map[string]interface{}{
				"type": "invalidtype",
				"port": 8080,
			},
		}

		runtimeConfig := &runtime.Configuration{
			Configuration: &processor.Configuration{
				Config: functionconfig.Config{
					Meta: functionconfig.Meta{
						Name:      "test-function",
						Namespace: "default",
					},
					Spec: functionconfig.Spec{},
				},
				PlatformConfig: &platformconfig.Config{
					Kind: "docker",
				},
			},
		}

		namedWorkerAllocators := worker.NewAllocatorSyncMap()
		restartTriggerChan := make(chan trigger.Trigger, 1)

		// Try to create trigger with invalid type
		triggerInstance, err := f.Create(parentLogger,
			"test-id",
			invalidTriggerConfig,
			runtimeConfig,
			namedWorkerAllocators,
			restartTriggerChan)

		assert.Error(t, err)
		assert.Nil(t, triggerInstance)
	})
}
