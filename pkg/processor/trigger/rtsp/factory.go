package rtsp

import (
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/trigger/rtsp/helpers"
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

	// Logger for this trigger
	triggerLogger := parentLogger.GetChild(triggerConfiguration.Kind)

	// Parse RTSP configuration
	configuration, err := helpers.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse RTSP trigger configuration")
	}

	// Worker allocator
	workerAllocator, err := f.GetWorkerAllocator(
		triggerConfiguration.WorkerAllocatorName,
		namedWorkerAllocators,
		func() (worker.Allocator, error) {
			return worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(
				triggerLogger,
				configuration.NumWorkers,
				runtimeConfiguration,
			)
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	// Create trigger
	triggerInstance, err := newTrigger(
		triggerLogger,
		workerAllocator,
		configuration,
		restartTriggerChan,
	)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create RTSP trigger")
	}

	triggerLogger.InfoWith("✓ RTSP trigger created",
		"rtspUrl", configuration.RTSPURL,
		"numWorkers", configuration.NumWorkers)

	return triggerInstance, nil
}

func init() {
	trigger.RegistrySingleton.Register("rtsp", &factory{})
}
