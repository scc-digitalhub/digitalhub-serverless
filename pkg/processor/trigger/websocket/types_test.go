/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package websocket

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/stretchr/testify/suite"
)

type WebsocketTypesTestSuite struct {
	suite.Suite
}

func (suite *WebsocketTypesTestSuite) SetupTest() {
}

func (suite *WebsocketTypesTestSuite) TestNewConfigurationValid() {
	triggerConfig := &functionconfig.Trigger{
		Kind: "websocket",
		Name: "test-websocket",
		Attributes: map[string]interface{}{
			"websocket_addr":      ":8080",
			"buffer_size":         8192,
			"chunk_bytes":         320000,
			"max_bytes":           2880000,
			"trim_bytes":          2240000,
			"processing_interval": 100,
			"is_stream":           true,
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
	suite.NoError(err)
	suite.NotNil(config)

	// Verify configuration values
	suite.Equal(":8080", config.WebSocketAddr)
	suite.Equal(8192, config.BufferSize)
	suite.Equal(320000, config.ChunkBytes)
	suite.Equal(2880000, config.MaxBytes)
	suite.Equal(2240000, config.TrimBytes)
	suite.Equal(100, int(config.ProcessingInterval))
	suite.True(config.IsStream)
}

func (suite *WebsocketTypesTestSuite) TestNewConfigurationDefaults() {
	triggerConfig := &functionconfig.Trigger{
		Kind: "websocket",
		Name: "test-websocket",
		Attributes: map[string]interface{}{
			"websocket_addr": ":8080",
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
	suite.NoError(err)
	suite.NotNil(config)

	// Verify defaults are applied
	suite.Equal(DefaultBufferSize, config.BufferSize)
	suite.Equal(DefaultChunkBytes, config.ChunkBytes)
	suite.Equal(DefaultMaxBytes, config.MaxBytes)
	suite.Equal(DefaultTrimBytes, config.TrimBytes)
	suite.Equal(time.Duration(DefaultProcessingInterval), config.ProcessingInterval)
	suite.Equal(DefaultIsStream, config.IsStream)
}

func (suite *WebsocketTypesTestSuite) TestNewConfigurationMissingWebSocketAddr() {
	triggerConfig := &functionconfig.Trigger{
		Kind: "websocket",
		Name: "test-websocket",
		Attributes: map[string]interface{}{
			"buffer_size": 8192,
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
	suite.Error(err)
	suite.Nil(config)
	suite.Contains(err.Error(), "websocket_addr is required")
}

func (suite *WebsocketTypesTestSuite) TestNewConfigurationInvalidAttributes() {
	triggerConfig := &functionconfig.Trigger{
		Kind: "websocket",
		Name: "test-websocket",
		Attributes: map[string]interface{}{
			"websocket_addr": ":8080",
			"buffer_size":    "invalid", // Should be int
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
	suite.Error(err) // mapstructure fails on invalid type conversion
	suite.Nil(config)
}

func (suite *WebsocketTypesTestSuite) TestNewConfigurationMapstructureDecodeError() {
	triggerConfig := &functionconfig.Trigger{
		Kind: "websocket",
		Name: "test-websocket",
		Attributes: map[string]interface{}{
			"websocket_addr": ":8080",
			"buffer_size":    "invalid_string", // This should cause mapstructure to fail
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
	suite.Error(err)
	suite.Nil(config)
	suite.Contains(err.Error(), "Failed to decode Websocket trigger attributes")
}

func (suite *WebsocketTypesTestSuite) TestConfigurationValidation() {
	// Test various edge cases for configuration values
	testCases := []struct {
		name        string
		attributes  map[string]interface{}
		expectError bool
	}{
		{
			name: "empty websocket_addr",
			attributes: map[string]interface{}{
				"websocket_addr": "",
			},
			expectError: true,
		},
		{
			name: "valid minimal config",
			attributes: map[string]interface{}{
				"websocket_addr": ":8080",
			},
			expectError: false,
		},
		{
			name: "large buffer size",
			attributes: map[string]interface{}{
				"websocket_addr": ":8080",
				"buffer_size":    1048576, // 1MB
			},
			expectError: false,
		},
		{
			name: "zero processing interval",
			attributes: map[string]interface{}{
				"websocket_addr":      ":8080",
				"processing_interval": 0,
			},
			expectError: false, // Should allow zero (though not recommended)
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			triggerConfig := &functionconfig.Trigger{
				Kind:       "websocket",
				Name:       "test-websocket",
				Attributes: tc.attributes,
			}

			runtimeConfig := &runtime.Configuration{
				Configuration: &processor.Configuration{
					Config: functionconfig.Config{
						Spec: functionconfig.Spec{
							Runtime: "python",
							Handler: "test_handler:handler",
						},
					},
				},
			}

			config, err := NewConfiguration("test-id", triggerConfig, runtimeConfig)
			if tc.expectError {
				suite.Error(err)
				suite.Nil(config)
			} else {
				suite.NoError(err)
				suite.NotNil(config)
			}
		})
	}
}

func TestWebsocketTypesTestSuite(t *testing.T) {
	suite.Run(t, new(WebsocketTypesTestSuite))
}
