/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package openinference

import (
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"
)

type factory struct {
	trigger.Factory
}

func (f *factory) Create(parentLogger logger.Logger,
	id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration,
	namedWorkerAllocators *worker.AllocatorSyncMap,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {

	// Create logger parent
	triggerLogger := parentLogger.GetChild(triggerConfiguration.Kind)

	// Parse configuration
	configuration, err := NewConfiguration(id, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse trigger configuration")
	}

	// Get or create worker allocator
	workerAllocator, err := f.GetWorkerAllocator(triggerConfiguration.WorkerAllocatorName,
		namedWorkerAllocators,
		func() (worker.Allocator, error) {
			return worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(triggerLogger,
				configuration.NumWorkers,
				runtimeConfiguration)
		})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// Create the trigger
	triggerInstance, err := newTrigger(triggerLogger,
		workerAllocator,
		configuration,
		restartTriggerChan)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create OpenInference trigger")
	}

	triggerLogger.InfoWith("✓ OpenInference trigger created",
		"modelName", configuration.ModelName,
		"modelVersion", configuration.ModelVersion,
		"enableREST", configuration.EnableREST,
		"enableGRPC", configuration.EnableGRPC,
		"numWorkers", configuration.NumWorkers)

	return triggerInstance, nil
}

// Register factory
func init() {
	trigger.RegistrySingleton.Register("openinference", &factory{})
}
