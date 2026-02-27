/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package openinference

import (
	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
)

const (
	DefaultRESTPort     = 8080
	DefaultGRPCPort     = 9000
	DefaultModelName    = "model"
	DefaultModelVersion = "1"
)

// TensorDef defines the shape and data type of a tensor
type TensorDef struct {
	Name     string  `mapstructure:"name"`
	DataType string  `mapstructure:"datatype"`
	Shape    []int64 `mapstructure:"shape"`
}

// Configuration for OpenInference trigger
type Configuration struct {
	trigger.Configuration

	// Server configuration
	RESTPort   int  `mapstructure:"rest_port"`
	GRPCPort   int  `mapstructure:"grpc_port"`
	EnableREST bool `mapstructure:"enable_rest"`
	EnableGRPC bool `mapstructure:"enable_grpc"`

	// Model configuration
	ModelName    string `mapstructure:"model_name"`
	ModelVersion string `mapstructure:"model_version"`

	// Tensor definitions
	InputTensors  []TensorDef `mapstructure:"input_tensors"`
	OutputTensors []TensorDef `mapstructure:"output_tensors"`
}

// NewConfiguration creates a new OpenInference trigger configuration
func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {

	newConfiguration := Configuration{
		RESTPort:     DefaultRESTPort,
		GRPCPort:     DefaultGRPCPort,
		EnableREST:   true,
		EnableGRPC:   true,
		ModelName:    DefaultModelName,
		ModelVersion: DefaultModelVersion,
	}

	// Create base configuration
	baseConfiguration, err := trigger.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create trigger configuration")
	}
	newConfiguration.Configuration = *baseConfiguration

	// Parse attributes
	if err := mapstructure.Decode(triggerConfiguration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode OpenInference trigger attributes")
	}

	// Validate configuration
	if !newConfiguration.EnableREST && !newConfiguration.EnableGRPC {
		return nil, errors.New("At least one of enable_rest or enable_grpc must be true")
	}

	if newConfiguration.ModelName == "" {
		return nil, errors.New("model_name is required")
	}

	return &newConfiguration, nil
}
