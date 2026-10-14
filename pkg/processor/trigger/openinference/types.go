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
	// Affine quantization params, for quantized models (int8/uint8): a client
	// receiving int8 cannot map the values back to reals without them. Per-axis
	// quantization yields more than one entry, indexed by QuantizedDimension.
	Scale              []float64 `mapstructure:"scale"`
	ZeroPoint          []int64   `mapstructure:"zero_point"`
	QuantizedDimension int64     `mapstructure:"quantized_dimension"`
}

// TensorMetadataProvider is implemented by runtimes that know their model's own
// signature (the TVM runtime reads it from metadata.json). The trigger prefers it
// over the tensors declared in the function config, so a function does not have to
// repeat — and risk contradicting — what the model already states. Runtimes that do
// not implement it keep the previous behaviour.
type TensorMetadataProvider interface {
	ModelTensors() (inputs, outputs []TensorDef)
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
