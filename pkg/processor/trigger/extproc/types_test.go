package extproc

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/stretchr/testify/assert"
)

func TestTypes(t *testing.T) {
	// Create base test configuration
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

	t.Run("NewConfiguration_Success", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind: "extproc",
			Attributes: map[string]interface{}{
				"type": "preprocessor",
				"port": 8080,
				"processingOptions": map[string]interface{}{
					"logStream":        true,
					"decompressBodies": true,
				},
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)

		// Verify configuration values
		assert.Equal(t, OperatorTypePre, config.Type)
		assert.Equal(t, 8080, config.Port)
		assert.NotNil(t, config.ProcessingOptions)
		assert.True(t, config.ProcessingOptions.LogStream)
		assert.True(t, config.ProcessingOptions.DecompressBodies)
	})

	t.Run("NewConfiguration_MissingType", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind: "extproc",
			Attributes: map[string]interface{}{
				"port": 8080,
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "Operator type not specified")
	})

	t.Run("NewConfiguration_MissingPort", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind: "extproc",
			Attributes: map[string]interface{}{
				"type": "preprocessor",
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "Port not specified")
	})

	t.Run("NewConfiguration_DefaultProcessingOptions", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind: "extproc",
			Attributes: map[string]interface{}{
				"type": "preprocessor",
				"port": 8080,
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.NotNil(t, config.ProcessingOptions)
		assert.Equal(t, "x-request-id", config.ProcessingOptions.RequestIdHeaderName)
		assert.True(t, config.ProcessingOptions.DecompressBodies)
		assert.True(t, config.ProcessingOptions.BufferStreamedBodies)
	})

	t.Run("NewConfiguration_CustomGracefulShutdownTimeout", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind: "extproc",
			Attributes: map[string]interface{}{
				"type":                    "preprocessor",
				"port":                    8080,
				"gracefulShutdownTimeout": 30,
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, 30, config.GracefulShutdownTimeout)
	})

	t.Run("NewConfiguration_CustomMaxConcurrentStreams", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind: "extproc",
			Attributes: map[string]interface{}{
				"type":                 "preprocessor",
				"port":                 8080,
				"maxConcurrentStreams": uint32(100),
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, uint32(100), config.MaxConcurrentStreams)
	})

	t.Run("NewConfiguration_AllOperatorTypes", func(t *testing.T) {
		operatorTypes := []OperatorType{
			OperatorTypePre,
			OperatorTypePost,
			OperatorTypeWrap,
			OperatorTypeObserve,
		}

		for _, opType := range operatorTypes {
			triggerConfig := &functionconfig.Trigger{
				Kind: "extproc",
				Attributes: map[string]interface{}{
					"type": opType,
					"port": 8080,
				},
			}

			config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
			assert.NoError(t, err)
			assert.NotNil(t, config)
			assert.Equal(t, opType, config.Type)
		}
	})
}
