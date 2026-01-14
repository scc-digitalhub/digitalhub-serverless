/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package websocket

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"
)

// websocket_trigger implements a Nuclio trigger that accepts WebSocket connections
// and forwards incoming data to workers, either in streaming or discrete mode.
type websocket_trigger struct {
	trigger.AbstractTrigger
	configuration *Configuration

	// either stream or discrete processor
	streamProcessor   *DataProcessorStream
	discreteProcessor *DataProcessorDiscrete

	wsServer *http.Server
	wsConn   *websocket.Conn
	wsLock   sync.Mutex

	stop chan struct{}
	wg   sync.WaitGroup
}

func newTrigger(
	logger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration,
	restartTriggerChan chan trigger.Trigger,
) (trigger.Trigger, error) {

	abstract, err := trigger.NewAbstractTrigger(
		logger,
		workerAllocator,
		&configuration.Configuration,
		"async",
		"websocket",
		configuration.Name,
		restartTriggerChan,
	)
	if err != nil {
		return nil, errors.Wrap(err, "abstract trigger")
	}

	ws_t := &websocket_trigger{
		AbstractTrigger: abstract,
		configuration:   configuration,
		stop:            make(chan struct{}),
	}
	ws_t.Trigger = ws_t
	return ws_t, nil
}

// initialize processors and launches server + dispatcher goroutines
func (ws_t *websocket_trigger) Start(_ functionconfig.Checkpoint) error {

	if ws_t.configuration.IsStream {
		ws_t.streamProcessor = NewDataProcessorStream(
			ws_t.configuration.ChunkBytes,
			ws_t.configuration.MaxBytes,
			ws_t.configuration.TrimBytes,
		)
		ws_t.streamProcessor.Start(time.Millisecond * time.Duration(ws_t.configuration.ProcessingInterval))
	} else {
		ws_t.discreteProcessor = NewDataProcessorDiscrete(time.Millisecond * time.Duration(ws_t.configuration.ProcessingInterval))
		ws_t.discreteProcessor.Start()
	}

	// Goroutine that dispatches processed events to Nuclio workers
	ws_t.wg.Add(1)
	go ws_t.eventDispatcher()

	// Goroutine that runs the HTTP/WebSocket server
	ws_t.wg.Add(1)
	go ws_t.startServer()

	return nil
}

// create HTTP server and register WebSocket handler
func (ws_t *websocket_trigger) startServer() {
	defer ws_t.wg.Done()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws_t.handleWS)

	ws_t.wsServer = &http.Server{
		Addr:    ws_t.configuration.WebSocketAddr,
		Handler: mux,
	}

	_ = ws_t.wsServer.ListenAndServe()
}

// upgrade HTTP connection to WebSocket and read incoming messages
func (ws_t *websocket_trigger) handleWS(w http.ResponseWriter, r *http.Request) {
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	conn, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	ws_t.wsLock.Lock()
	ws_t.wsConn = conn
	ws_t.wsLock.Unlock()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if err == io.EOF {
				return
			}
			return
		}

		if ws_t.configuration.IsStream {
			ws_t.streamProcessor.Push(data)
		} else {
			ws_t.discreteProcessor.Push(data)
		}
	}
}

// wait for processed events and submit them to Nuclio workers
func (ws_t *websocket_trigger) eventDispatcher() {
	defer ws_t.wg.Done()

	for {
		select {
		case <-ws_t.stop:
			return
		// receive processed events from data processor
		case ev := <-ws_t.getOutput():
			ws_t.process(ev)
		}
	}
}

// return the output channel of the active processor
func (ws_t *websocket_trigger) getOutput() <-chan *Event {
	if ws_t.configuration.IsStream {
		return ws_t.streamProcessor.Output()
	}
	return ws_t.discreteProcessor.Output()
}

// submit event to Nuclio worker and send response back via WebSocket
func (ws_t *websocket_trigger) process(ev *Event) {
	w, err := ws_t.WorkerAllocator.Allocate(5 * time.Second)
	if err != nil {
		return
	}
	defer ws_t.WorkerAllocator.Release(w)

	resp, err := ws_t.SubmitEventToWorker(ws_t.Logger, w, ev)
	if err != nil {
		return
	}

	if r, ok := resp.(nuclio.Response); ok {
		ws_t.wsLock.Lock()
		if ws_t.wsConn != nil {
			_ = ws_t.wsConn.WriteMessage(websocket.TextMessage, r.Body)
		}
		ws_t.wsLock.Unlock()
	}
}

// shuts down processors, HTTP server, and background goroutines
func (ws_t *websocket_trigger) Stop(bool) (functionconfig.Checkpoint, error) {
	close(ws_t.stop)
	if ws_t.configuration.IsStream {
		ws_t.streamProcessor.Stop()
	} else {
		ws_t.discreteProcessor.Stop()
	}
	if ws_t.wsServer != nil {
		_ = ws_t.wsServer.Shutdown(context.TODO())
	}
	ws_t.wg.Wait()
	return nil, nil
}

func (ws_t *websocket_trigger) GetConfig() map[string]any {
	return common.StructureToMap(ws_t.configuration)
}
