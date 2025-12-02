package websocket

import (
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
	events        []Event
	stopChan      chan struct{}
	wg            sync.WaitGroup
	processor     *AudioProcessor

	wsServer   *http.Server
	wsConn     *websocket.Conn
	wsLock     sync.Mutex
	wsUpgrader websocket.Upgrader
}

func newTrigger(logger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {

	abstractTrigger, err := trigger.NewAbstractTrigger(
		logger,
		workerAllocator,
		&configuration.Configuration,
		"async",
		"websocket",
		configuration.Name,
		restartTriggerChan)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract trigger")
	}

	newTrigger := &websocket_trigger{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
		stopChan:        make(chan struct{}),
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	newTrigger.Trigger = newTrigger
	newTrigger.allocateEvents(1)
	return newTrigger, nil
}

func (ws_t *websocket_trigger) Start(checkpoint functionconfig.Checkpoint) error {
	ws_t.Logger.InfoWith("Starting WebSocket-only trigger",
		"websocketAddr", ws_t.configuration.WebSocketAddr)

	ws_t.processor = NewAudioProcessor(
		ws_t.configuration.SampleRate,
		ws_t.configuration.ChunkDurationSeconds,
		ws_t.configuration.MaxBufferSeconds,
		ws_t.configuration.TrimSeconds,
		ws_t.configuration.AccumulateBuffer)

	ws_t.wg.Add(1)
	go ws_t.startWebSocketServer()

	return nil
}

func (ws_t *websocket_trigger) startWebSocketServer() {
	defer ws_t.wg.Done()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws_t.handleWebSocketUpgrade)

	ws_t.wsServer = &http.Server{
		Addr:    ws_t.configuration.WebSocketAddr,
		Handler: mux,
	}

	ws_t.Logger.InfoWith("✓ WebSocket server listening",
		"addr", ws_t.configuration.WebSocketAddr)

	if err := ws_t.wsServer.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		ws_t.Logger.ErrorWith("✗ WebSocket server error", "error", err)
	}
}

func (ws_t *websocket_trigger) handleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) {
	ws_t.wsLock.Lock()
	defer ws_t.wsLock.Unlock()

	if ws_t.wsConn != nil {
		http.Error(w, "Client already connected", http.StatusForbidden)
		return
	}

	conn, err := ws_t.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		ws_t.Logger.ErrorWith("Failed to upgrade WebSocket", "error", err)
		return
	}

	ws_t.wsConn = conn
	ws_t.Logger.InfoWith("✓ WebSocket client connected")

	ws_t.wg.Add(1)
	go ws_t.handleIncomingMessages(conn)
}

func (ws_t *websocket_trigger) handleIncomingMessages(conn *websocket.Conn) {
	defer ws_t.wg.Done()

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err) || err == io.EOF {
				ws_t.Logger.InfoWith("WebSocket closed by client")
			} else {
				ws_t.Logger.WarnWith("WebSocket read error", "error", err)
			}
			ws_t.wsConn = nil
			return
		}

		if msgType != websocket.BinaryMessage {
			continue
		}

		chunks := ws_t.processor.AddPCM(data)
		if len(chunks) == 0 {
			continue
		}

		ws_t.processor.lock.Lock()
		rolling := make([]byte, len(ws_t.processor.buffer))
		copy(rolling, ws_t.processor.buffer)
		ws_t.processor.lock.Unlock()

		event := &Event{
			body:      rolling,
			timestamp: time.Now(),
			attributes: map[string]interface{}{
				"buffer-size": len(rolling),
				"chunks":      len(chunks),
			},
		}

		workerInstance, err := ws_t.WorkerAllocator.Allocate(time.Second * 5)
		if err != nil {
			ws_t.Logger.WarnWith("Failed to allocate worker", "error", err)
			continue
		}

		response, processErr := ws_t.SubmitEventToWorker(
			ws_t.Logger, workerInstance, event)

		ws_t.WorkerAllocator.Release(workerInstance)

		if processErr != nil {
			ws_t.Logger.WarnWith("Handler execution error", "error", processErr)
			continue
		}

		if typed, ok := response.(nuclio.Response); ok {
			ws_t.wsLock.Lock()
			if ws_t.wsConn != nil {
				ws_t.wsConn.WriteMessage(websocket.TextMessage, typed.Body)
			}
			ws_t.wsLock.Unlock()
		}
	}
}

func (ws_t *websocket_trigger) Stop(force bool) (functionconfig.Checkpoint, error) {
	close(ws_t.stopChan)

	if ws_t.wsServer != nil {
		ws_t.wsServer.Shutdown(nil) //nolint
	}

	ws_t.wsLock.Lock()
	if ws_t.wsConn != nil {
		ws_t.wsConn.Close()
	}
	ws_t.wsLock.Unlock()

	ws_t.wg.Wait()
	ws_t.Logger.InfoWith("✓ WebSocket trigger stopped")
	return nil, nil
}

func (ws_t *websocket_trigger) GetConfig() map[string]interface{} {
	return common.StructureToMap(ws_t.configuration)
}

func (ws_t *websocket_trigger) allocateEvents(size int) {
	ws_t.events = make([]Event, size)
	for i := range ws_t.events {
		ws_t.events[i] = Event{}
	}
}
