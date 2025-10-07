/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/
package extproc

import (
	"strconv"
	"time"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"
)

type extproc struct {
	trigger.AbstractTrigger
	events         []Event
	status         status.Status
	activeContexts []*Event
	configuration  *Configuration
	timeouts       []uint64 // flag of worker is in timeout
	answering      []uint64 // flag the worker is answering
}

func newTrigger(logger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {

	numWorkers := len(workerAllocator.GetWorkers())

	abstractTrigger, err := trigger.NewAbstractTrigger(logger,
		workerAllocator,
		&configuration.Configuration,
		"sync",
		"extproc",
		configuration.Name,
		restartTriggerChan)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := extproc{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
		status:          status.Initializing,
		activeContexts:  make([]*Event, numWorkers),
		timeouts:        make([]uint64, numWorkers),
		answering:       make([]uint64, numWorkers),
	}

	newTrigger.Trigger = &newTrigger
	newTrigger.allocateEvents(numWorkers)
	return &newTrigger, nil
}

func (ep *extproc) Start(checkpoint functionconfig.Checkpoint) error {
	ep.Logger.InfoWith("Starting",
		"listenAddress", ep.configuration.URL,
		"operatorType", ep.configuration.Type,
		"GracefulShutdownTimeout", ep.configuration.GracefulShutdownTimeout,
		"MaxConcurrentStreams", ep.configuration.MaxConcurrentStreams)

	serverOptions := DefaultServerOptions()
	if ep.configuration.GracefulShutdownTimeout != 0 {
		serverOptions.GracefulShutdownTimeout = ep.configuration.GracefulShutdownTimeout
	}
	if ep.configuration.MaxConcurrentStreams != 0 {
		serverOptions.MaxConcurrentStreams = ep.configuration.MaxConcurrentStreams
	}

	switch ep.configuration.Type {
	case OperatorTypePre:
		ep.Logger.Info("Starting preprocessor server")
		proc := &PreProcessor{}
		proc.Init(ep.configuration.ProcessingOptions, nil, ep)
		go ServeWithOptions(ep.configuration.Port, serverOptions, proc)
	case OperatorTypePost:
		ep.Logger.Info("Starting postprocessor server")
		proc := &PostProcessor{}
		proc.Init(ep.configuration.ProcessingOptions, nil, ep)
		go ServeWithOptions(ep.configuration.Port, serverOptions, proc)
	case OperatorTypeWrap:
		ep.Logger.Info("Starting wrapprocessor server")
		proc := &WrapProcessor{}
		proc.Init(ep.configuration.ProcessingOptions, nil, ep)
		go ServeWithOptions(ep.configuration.Port, serverOptions, proc)
	case OperatorTypeObserve:
		ep.Logger.Info("Starting observeprocessor server")
		proc := &ObserveProcessor{}
		proc.Init(ep.configuration.ProcessingOptions, nil, ep)
		go ServeWithOptions(ep.configuration.Port, serverOptions, proc)
	default:
		return errors.New("Unknown operator type: " + string(ep.configuration.Type))
	}

	ep.status = status.Ready
	return nil
}

func (ep *extproc) Stop(force bool) (functionconfig.Checkpoint, error) {
	ep.Logger.Debug("Stopping extproc trigger")

	ep.status = status.Stopped

	// TODO : stop server gracefully

	return nil, nil
}

func (ep *extproc) AllocateWorkerAndSubmitEvent(req *Event,
	functionLogger logger.Logger,
	timeout time.Duration) (response interface{}, timedOut bool, submitError error, processError error) {

	var workerInstance *worker.Worker

	defer ep.HandleSubmitPanic(workerInstance, &submitError)

	// allocate a worker
	workerInstance, err := ep.WorkerAllocator.Allocate(timeout)
	if err != nil {
		ep.UpdateStatistics(false)
		return nil, false, errors.Wrap(err, "Failed to allocate worker"), nil
	}

	// use the event @ the worker index
	// TODO: event already used?
	workerIndex := workerInstance.GetIndex()
	if workerIndex < 0 || workerIndex >= len(ep.events) {
		ep.WorkerAllocator.Release(workerInstance)
		return nil, false, errors.Errorf("Worker index (%d) bigger than size of event pool (%d)", workerIndex, len(ep.events)), nil
	}

	ep.activeContexts[workerIndex] = req
	ep.timeouts[workerIndex] = 0
	ep.answering[workerIndex] = 0
	event := &ep.events[workerIndex]
	event.ctx = req.ctx
	event.Body = req.Body

	// submit to worker
	response, processError = ep.SubmitEventToWorker(functionLogger, workerInstance, event)
	// release worker when we're done
	ep.WorkerAllocator.Release(workerInstance)

	if ep.timeouts[workerIndex] == 1 {
		return nil, true, nil, nil
	}

	ep.answering[workerIndex] = 1
	ep.activeContexts[workerIndex] = nil

	return response, false, nil, processError
}

func (ep *extproc) TimeoutWorker(worker *worker.Worker) error {
	workerIndex := worker.GetIndex()
	if workerIndex < 0 || workerIndex >= len(ep.activeContexts) {
		return errors.Errorf("Worker %d out of range", workerIndex)
	}

	ep.timeouts[workerIndex] = 1
	time.Sleep(time.Millisecond) // Let worker do it's thing
	if ep.answering[workerIndex] == 1 {
		return errors.Errorf("Worker %d answered the request", workerIndex)
	}

	ctx := ep.activeContexts[workerIndex]
	if ctx == nil {
		return errors.Errorf("Worker %d answered the request", workerIndex)
	}

	ep.activeContexts[workerIndex] = nil
	return nil
}

func (ep *extproc) handleRequest(ctx *RequestContext, body []byte) (*nuclio.Response, error) {
	var functionLogger logger.Logger

	event := Event{
		ctx:  ctx,
		Body: body,
	}

	response, timedOut, submitError, processError := ep.AllocateWorkerAndSubmitEvent(&event,
		functionLogger,
		time.Duration(*ep.configuration.WorkerAvailabilityTimeoutMilliseconds)*time.Millisecond)

	if timedOut {
		return nil, nuclio.ErrRequestTimeout
	}

	// Clear active context in case of error
	if submitError != nil || processError != nil {
		for i, activeCtx := range ep.activeContexts {
			if activeCtx.ctx == ctx {
				ep.activeContexts[i] = nil
				break
			}
		}
	}

	// if we failed to submit the event to a worker
	if submitError != nil {
		return nil, submitError
	}

	if processError != nil {
		return nil, submitError
	}

	// format the response into the context, based on its type
	switch typedResponse := response.(type) {
	case nuclio.Response:
		// if the response contains a processing status, use it as the response status code ignoring the status code which is default 200
		if typedResponse.Headers != nil {
			val, ok := typedResponse.Headers["X-Processing-Status"]
			if !ok {
				typedResponse.StatusCode = 0
			} else {
				delete(typedResponse.Headers, "X-Processing-Status")
				status, statusErr := strconv.Atoi(string(val.(string)))
				if statusErr == nil {
					typedResponse.StatusCode = status
				}
			}
		}
		return &typedResponse, nil

	case []byte:
		res := &nuclio.Response{
			StatusCode: 0,
			Body:       typedResponse,
		}
		return res, nil

	case string:
		res := &nuclio.Response{
			StatusCode: 0,
			Body:       []byte(typedResponse), // TODO: this is not a string, but a byte array, should be fixed in nucliotypedResponse,
		}
		return res, nil
	}
	return nil, errors.New("unknown response type")
}

func (ep *extproc) GetConfig() map[string]interface{} {
	return common.StructureToMap(ep.configuration)
}

func (ep *extproc) allocateEvents(size int) {
	ep.events = make([]Event, size)
	for i := 0; i < size; i++ {
		ep.events[i] = Event{}
	}
}

func (ep *extproc) HandleEvent(ctx *RequestContext, body []byte) (*EventResponse, error) {
	res, err := ep.handleRequest(ctx, body)
	er := EventResponse{}
	if err != nil {
		return nil, err
	}
	if res != nil {
		er.Status = res.StatusCode
		er.Headers = make(map[string]string)
		for headerKey, headerValue := range res.Headers {
			switch typedHeaderValue := headerValue.(type) {
			case string:
				er.Headers[headerKey] = typedHeaderValue
			case int:
				er.Headers[headerKey] = strconv.Itoa(typedHeaderValue)
			}
		}
		er.Body = res.Body
	}
	return &er, nil
}
