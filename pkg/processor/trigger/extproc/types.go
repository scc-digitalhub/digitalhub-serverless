/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package extproc

import (
	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/errors"
)

type Configuration struct {
	trigger.Configuration
	Type                    OperatorType       `json:"type"`
	Port                    int                `json:"port"`
	GracefulShutdownTimeout int                `json:"gracefulShutdownTimeout,omitempty"`
	MaxConcurrentStreams    uint32             `json:"maxConcurrentStreams,omitempty"`
	ProcessingOptions       *ProcessingOptions `json:"processingOptions"`
}

type OperatorType string

const (
	OperatorTypePre     OperatorType = "preprocessor"
	OperatorTypePost    OperatorType = "postprocessor"
	OperatorTypeWrap    OperatorType = "wrapprocessor"
	OperatorTypeObserve OperatorType = "observeprocessor"
)

func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	baseConfiguration, err := trigger.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create trigger configuration")
	}
	newConfiguration.Configuration = *baseConfiguration

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode trigger configuration")
	}

	// validate required fields
	if newConfiguration.Type == "" {
		return nil, errors.New("Operator type not specified")
	}
	if newConfiguration.Port == 0 {
		return nil, errors.New("Port not specified")
	}

	if newConfiguration.ProcessingOptions == nil {
		newConfiguration.ProcessingOptions = NewDefaultOptions()
	}

	return &newConfiguration, nil
}
