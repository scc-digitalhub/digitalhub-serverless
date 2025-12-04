package websocket

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
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

	triggerLogger := parentLogger.GetChild(triggerConfiguration.Kind)

	configuration, err := NewConfiguration(id, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse Websocket trigger configuration")
	}

	workerAllocator, err := f.GetWorkerAllocator(
		triggerConfiguration.WorkerAllocatorName,
		namedWorkerAllocators,
		func() (worker.Allocator, error) {
			return worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(
				triggerLogger,
				configuration.NumWorkers,
				runtimeConfiguration)
		})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	triggerInstance, err := newTrigger(
		triggerLogger,
		workerAllocator,
		configuration,
		restartTriggerChan)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create Websocket trigger")
	}

	triggerLogger.InfoWith("✓ Websocket trigger created",
		"websocketAddr", configuration.WebSocketAddr,
		"numWorkers", configuration.NumWorkers)

	return triggerInstance, nil
}

func init() {
	trigger.RegistrySingleton.Register("websocket", &factory{})
}
