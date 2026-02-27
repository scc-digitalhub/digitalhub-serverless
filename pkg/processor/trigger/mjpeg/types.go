/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package mjpeg

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
)

const (
	DefaultProcessingFactor = 1 // Process every frame by default
)

// SinkConfiguration holds the sink configuration for the trigger
type SinkConfiguration struct {
	Kind       string                 `mapstructure:"kind"`
	Attributes map[string]interface{} `mapstructure:"attributes"`
}

// Configuration holds the MJPEG trigger configuration
type Configuration struct {
	trigger.Configuration
	URL              string             `mapstructure:"url"`
	ProcessingFactor int                `mapstructure:"processing_factor"`
	Sink             *SinkConfiguration `mapstructure:"sink"`
}

// NewConfiguration creates a new MJPEG trigger configuration
func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {

	newConfiguration := Configuration{
		ProcessingFactor: DefaultProcessingFactor,
	}

	// create base configuration
	baseConfiguration, err := trigger.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create trigger configuration")
	}
	newConfiguration.Configuration = *baseConfiguration

	// parse attributes
	if err := mapstructure.Decode(triggerConfiguration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode MJPEG trigger attributes")
	}

	// validate required fields
	if newConfiguration.URL == "" {
		return nil, errors.New("url is required for MJPEG trigger")
	}

	// validate processing factor
	if newConfiguration.ProcessingFactor < 1 {
		return nil, errors.New("processing_factor must be >= 1")
	}

	return &newConfiguration, nil
}
