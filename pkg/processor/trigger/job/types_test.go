package job

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger/cron"
	"github.com/stretchr/testify/assert"
)

func TestTypes(t *testing.T) {
	// Create base runtime configuration
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
		cronEvent := cron.Event{
			Body: "test body",
			Headers: map[string]interface{}{
				"Content-Type": "text/plain",
			},
		}

		triggerConfig := &functionconfig.Trigger{
			Kind: "job",
			Attributes: map[string]interface{}{
				"event": cronEvent,
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, cronEvent.Body, config.Event.Body)
		assert.Equal(t, cronEvent.Headers["Content-Type"], config.Event.Headers["Content-Type"])
	})

	t.Run("NewConfiguration_AllEventFields", func(t *testing.T) {
		cronEvent := cron.Event{
			Body:   "test body",
			Path:   "/test/path",
			Method: "POST",
			Headers: map[string]interface{}{
				"Content-Type": "application/json",
				"X-Test":       "test-value",
			},
		}

		triggerConfig := &functionconfig.Trigger{
			Kind: "job",
			Attributes: map[string]interface{}{
				"event": cronEvent,
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, cronEvent.Body, config.Event.Body)
		assert.Equal(t, cronEvent.Path, config.Event.Path)
		assert.Equal(t, cronEvent.Method, config.Event.Method)
		assert.Equal(t, cronEvent.Headers, config.Event.Headers)
	})

	t.Run("NewConfiguration_MergeWithBaseConfig", func(t *testing.T) {
		cronEvent := cron.Event{
			Body: "test body",
		}

		triggerConfig := &functionconfig.Trigger{
			Kind: "job",
			Name: "test-trigger",
			Attributes: map[string]interface{}{
				"event": cronEvent,
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "test-trigger", config.Name)
		assert.Equal(t, cronEvent.Body, config.Event.Body)
	})

	t.Run("NewConfiguration_AttributeTypeConversion", func(t *testing.T) {
		// Test that attributes are properly converted from map[string]interface{}
		triggerConfig := &functionconfig.Trigger{
			Kind: "job",
			Attributes: map[string]interface{}{
				"event": map[string]interface{}{
					"body": "test body",
					"headers": map[string]interface{}{
						"numeric": 123,
						"boolean": true,
						"string":  "value",
					},
				},
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "test body", config.Event.Body)
		assert.Equal(t, 123, config.Event.Headers["numeric"])
		assert.Equal(t, true, config.Event.Headers["boolean"])
		assert.Equal(t, "value", config.Event.Headers["string"])
	})

	t.Run("NewConfiguration_EmptyAttributes", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind:       "job",
			Attributes: map[string]interface{}{},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Empty(t, config.Event.Body)
		assert.Empty(t, config.Event.Path)
		assert.Empty(t, config.Event.Method)
		assert.Nil(t, config.Event.Headers)
	})

	t.Run("NewConfiguration_InvalidAttributes", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind: "job",
			Attributes: map[string]interface{}{
				"event": 123, // Invalid type for event
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to decode attributes")
		assert.Nil(t, config)
	})
}
