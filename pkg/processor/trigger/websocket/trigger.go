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

type websocket_trigger struct {
	trigger.AbstractTrigger
	configuration *Configuration

	processor *DataProcessor

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

func (ws_t *websocket_trigger) Start(_ functionconfig.Checkpoint) error {
	ws_t.processor = NewDataProcessor(
		ws_t.configuration.ChunkBytes,
		ws_t.configuration.MaxBytes,
		ws_t.configuration.TrimBytes,
		ws_t.configuration.SleepTime,
		ws_t.configuration.IsStream,
	)

	ws_t.processor.Start()

	ws_t.wg.Add(1)
	go ws_t.eventDispatcher()

	ws_t.wg.Add(1)
	go ws_t.startServer()

	return nil
}

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
		ws_t.processor.manageBuffer(data)
	}
}

func (ws_t *websocket_trigger) eventDispatcher() {
	defer ws_t.wg.Done()

	for {
		select {
		case <-ws_t.stop:
			return
		case event := <-ws_t.processor.Output():
			ws_t.process(event)
		}
	}
}

func (ws_t *websocket_trigger) process(event *Event) {
	w, err := ws_t.WorkerAllocator.Allocate(5 * time.Second)
	if err != nil {
		return
	}
	defer ws_t.WorkerAllocator.Release(w)

	resp, err := ws_t.SubmitEventToWorker(ws_t.Logger, w, event)
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

func (ws_t *websocket_trigger) Stop(bool) (functionconfig.Checkpoint, error) {
	close(ws_t.stop)
	ws_t.processor.Stop()
	if ws_t.wsServer != nil {
		_ = ws_t.wsServer.Shutdown(context.TODO())
	}
	ws_t.wg.Wait()
	return nil, nil
}

func (ws_t *websocket_trigger) GetConfig() map[string]any {
	return common.StructureToMap(ws_t.configuration)
}
