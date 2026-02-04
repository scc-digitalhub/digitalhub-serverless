/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package mjpeg

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
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
		triggerConfig := &functionconfig.Trigger{
			Kind: "mjpeg",
			Attributes: map[string]interface{}{
				"url":               "http://example.com/stream.mjpg",
				"processing_factor": 2,
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "http://example.com/stream.mjpg", config.URL)
		assert.Equal(t, 2, config.ProcessingFactor)
	})

	t.Run("NewConfiguration_DefaultProcessingFactor", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind: "mjpeg",
			Attributes: map[string]interface{}{
				"url": "http://example.com/stream.mjpg",
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "http://example.com/stream.mjpg", config.URL)
		assert.Equal(t, DefaultProcessingFactor, config.ProcessingFactor)
	})

	t.Run("NewConfiguration_MissingURL", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind:       "mjpeg",
			Attributes: map[string]interface{}{},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "url is required")
	})

	t.Run("NewConfiguration_InvalidProcessingFactor", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind: "mjpeg",
			Attributes: map[string]interface{}{
				"url":               "http://example.com/stream.mjpg",
				"processing_factor": 0,
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "processing_factor must be >= 1")
	})

	t.Run("NewConfiguration_NegativeProcessingFactor", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind: "mjpeg",
			Attributes: map[string]interface{}{
				"url":               "http://example.com/stream.mjpg",
				"processing_factor": -1,
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "processing_factor must be >= 1")
	})

	t.Run("NewConfiguration_ProcessingFactor10", func(t *testing.T) {
		triggerConfig := &functionconfig.Trigger{
			Kind: "mjpeg",
			Attributes: map[string]interface{}{
				"url":               "http://camera.local/video.mjpg",
				"processing_factor": 10,
			},
		}

		config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "http://camera.local/video.mjpg", config.URL)
		assert.Equal(t, 10, config.ProcessingFactor)
	})
}
