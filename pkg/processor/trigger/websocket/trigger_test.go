package websocket

import (
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type mockWorkerAllocator struct {
	allocateFunc func(timeout time.Duration) (*worker.Worker, error)
	releaseFunc  func(*worker.Worker)
	workers      []*worker.Worker
}

func newMockWorkerAllocator() (worker.Allocator, error) {
	return &mockWorkerAllocator{
		workers: []*worker.Worker{{}},
	}, nil
}

func (m *mockWorkerAllocator) Allocate(timeout time.Duration) (*worker.Worker, error) {
	if m.allocateFunc != nil {
		return m.allocateFunc(timeout)
	}
	// Create a worker with a mock runtime
	mockRuntime := &mockRuntime{}
	worker, err := worker.NewWorker(nil, 0, mockRuntime)
	if err != nil {
		return nil, err
	}
	return worker, nil
}

func (m *mockWorkerAllocator) Release(w *worker.Worker) {
	if m.releaseFunc != nil {
		m.releaseFunc(w)
	}
}

func (m *mockWorkerAllocator) Stop() {}

func (m *mockWorkerAllocator) GetNumWorkers() int {
	return len(m.workers)
}

func (m *mockWorkerAllocator) GetWorkers() []*worker.Worker {
	return m.workers
}

func (m *mockWorkerAllocator) GetNumWorkersAvailable() int {
	return len(m.workers)
}

func (m *mockWorkerAllocator) GetStatistics() *worker.AllocatorStatistics {
	return &worker.AllocatorStatistics{}
}

func (m *mockWorkerAllocator) IsTerminated() bool {
	return false
}

func (m *mockWorkerAllocator) Shareable() bool {
	return false
}

func (m *mockWorkerAllocator) SignalContinue() error {
	return nil
}

func (m *mockWorkerAllocator) SignalDraining() error {
	return nil
}

func (m *mockWorkerAllocator) SignalTermination() error {
	return nil
}

type mockRuntime struct{}

func (m *mockRuntime) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (interface{}, error) {
	return nuclio.Response{
		Body: []byte("mock response"),
	}, nil
}

func (m *mockRuntime) ProcessBatch(batch []nuclio.Event, functionLogger logger.Logger) ([]*runtime.ResponseWithErrors, error) {
	responses := make([]*runtime.ResponseWithErrors, len(batch))
	for i := range batch {
		responses[i] = &runtime.ResponseWithErrors{
			Response: nuclio.Response{
				Body: []byte("mock batch response"),
			},
		}
	}
	return responses, nil
}

func (m *mockRuntime) GetFunctionLogger() logger.Logger {
	return nil
}

func (m *mockRuntime) GetStatistics() *runtime.Statistics {
	return &runtime.Statistics{}
}

func (m *mockRuntime) GetConfiguration() *runtime.Configuration {
	return &runtime.Configuration{}
}

func (m *mockRuntime) SetStatus(newStatus status.Status) {}

func (m *mockRuntime) GetStatus() status.Status {
	return status.Ready
}

func (m *mockRuntime) Start() error {
	return nil
}

func (m *mockRuntime) Stop() error {
	return nil
}

func (m *mockRuntime) Restart() error {
	return nil
}

func (m *mockRuntime) SupportsRestart() bool {
	return false
}

func (m *mockRuntime) Drain() error {
	return nil
}

func (m *mockRuntime) Continue() error {
	return nil
}

func (m *mockRuntime) Terminate() error {
	return nil
}

func (m *mockRuntime) GetControlMessageBroker() controlcommunication.ControlMessageBroker {
	return nil
}

type WebsocketTriggerTestSuite struct {
	suite.Suite
	logger                logger.Logger
	namedWorkerAllocators *worker.AllocatorSyncMap
}

func (suite *WebsocketTriggerTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.namedWorkerAllocators = worker.NewAllocatorSyncMap()
	allocator, _ := newMockWorkerAllocator()
	suite.namedWorkerAllocators.Store("mock-allocator", allocator)
}

func (suite *WebsocketTriggerTestSuite) TestTriggerCreation() {
	tests := []struct {
		name          string
		triggerConfig *functionconfig.Trigger
		runtimeConfig *runtime.Configuration
		wantErr       bool
	}{
		{
			name: "valid discrete configuration",
			triggerConfig: &functionconfig.Trigger{
				Kind:                "websocket",
				Name:                "test-websocket",
				WorkerAllocatorName: "mock-allocator",
				Attributes: map[string]interface{}{
					"websocket_addr":      ":8080",
					"buffer_size":         4096,
					"processing_interval": 2000,
					"is_stream":           false,
				},
			},
			runtimeConfig: &runtime.Configuration{
				Configuration: &processor.Configuration{
					Config: functionconfig.Config{
						Spec: functionconfig.Spec{
							Runtime: "python",
							Handler: "test_handler:handler",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid stream configuration",
			triggerConfig: &functionconfig.Trigger{
				Kind:                "websocket",
				Name:                "test-websocket-stream",
				WorkerAllocatorName: "mock-allocator",
				Attributes: map[string]interface{}{
					"websocket_addr":      ":8080",
					"chunk_bytes":         160000,
					"max_bytes":           1440000,
					"trim_bytes":          1120000,
					"processing_interval": 20,
					"is_stream":           true,
				},
			},
			runtimeConfig: &runtime.Configuration{
				Configuration: &processor.Configuration{
					Config: functionconfig.Config{
						Spec: functionconfig.Spec{
							Runtime: "python",
							Handler: "test_handler:handler",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing websocket_addr",
			triggerConfig: &functionconfig.Trigger{
				Kind:                "websocket",
				Name:                "test-websocket",
				WorkerAllocatorName: "mock-allocator",
				Attributes: map[string]interface{}{
					"buffer_size":         4096,
					"processing_interval": 2000,
					"is_stream":           false,
				},
			},
			runtimeConfig: &runtime.Configuration{
				Configuration: &processor.Configuration{
					Config: functionconfig.Config{
						Spec: functionconfig.Spec{
							Runtime: "python",
							Handler: "test_handler:handler",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			f := &factory{}
			triggerInstance, err := f.Create(suite.logger, "test-websocket", tt.triggerConfig, tt.runtimeConfig, suite.namedWorkerAllocators, nil)

			if tt.wantErr {
				suite.Error(err)
				suite.Nil(triggerInstance)
				return
			}

			suite.NoError(err)
			suite.NotNil(triggerInstance)

			// Verify it's a websocket trigger
			_, ok := triggerInstance.(*websocket_trigger)
			suite.True(ok)
		})
	}
}

func (suite *WebsocketTriggerTestSuite) TestConfigurationDefaults() {
	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket",
		WorkerAllocatorName: "mock-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr": ":8080",
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	f := &factory{}
	triggerInstance, err := f.Create(suite.logger, "test-websocket", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.NoError(err)
	suite.NotNil(triggerInstance)

	wsTrigger := triggerInstance.(*websocket_trigger)
	config := wsTrigger.configuration

	// Check defaults are applied
	suite.Equal(DefaultBufferSize, config.BufferSize)
	suite.Equal(DefaultChunkBytes, config.ChunkBytes)
	suite.Equal(DefaultMaxBytes, config.MaxBytes)
	suite.Equal(DefaultTrimBytes, config.TrimBytes)
	suite.Equal(time.Duration(DefaultProcessingInterval), config.ProcessingInterval)
	suite.Equal(DefaultIsStream, config.IsStream)
	suite.Equal(":8080", config.WebSocketAddr)
}

func (suite *WebsocketTriggerTestSuite) TestConnectionManagement() {
	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket",
		WorkerAllocatorName: "mock-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr":      ":0", // Use port 0 for auto-assignment
			"processing_interval": 10,
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	f := &factory{}
	triggerInstance, err := f.Create(suite.logger, "test-websocket", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.NoError(err)

	wsTrigger := triggerInstance.(*websocket_trigger)

	// Test initial state
	suite.Equal(0, len(wsTrigger.conns))
	suite.Equal(1, wsTrigger.maxClients) // numWorkers = 1 by default

	// Simulate adding connections
	mockConn1 := &websocket.Conn{}
	mockConn2 := &websocket.Conn{}

	wsTrigger.connLock.Lock()
	wsTrigger.conns[mockConn1] = struct{}{}
	wsTrigger.conns[mockConn2] = struct{}{}
	wsTrigger.connLock.Unlock()

	suite.Equal(2, len(wsTrigger.conns))

	// Test removing connections
	wsTrigger.connLock.Lock()
	delete(wsTrigger.conns, mockConn1)
	wsTrigger.connLock.Unlock()

	suite.Equal(1, len(wsTrigger.conns))
}

func (suite *WebsocketTriggerTestSuite) TestEventProcessing() {
	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket",
		WorkerAllocatorName: "mock-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr":      ":0",
			"processing_interval": 10,
			"is_stream":           false,
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	f := &factory{}
	triggerInstance, err := f.Create(suite.logger, "test-websocket", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.NoError(err)

	wsTrigger := triggerInstance.(*websocket_trigger)

	// Create a test event
	testEvent := &Event{
		body:      []byte("test message"),
		timestamp: time.Now(),
	}

	// Mock the SubmitEventToWorker method by setting up the abstract trigger
	// This is tricky to test directly, so we'll test the process method indirectly
	// by checking that it doesn't panic and handles nil events

	// Test with nil event
	wsTrigger.process(nil) // Should not panic

	// Test with valid event (will fail on worker allocation in test, but shouldn't panic)
	wsTrigger.process(testEvent) // Should not panic
}

func (suite *WebsocketTriggerTestSuite) TestStop() {
	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket",
		WorkerAllocatorName: "mock-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr":      ":0",
			"processing_interval": 10,
			"is_stream":           false,
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	f := &factory{}
	triggerInstance, err := f.Create(suite.logger, "test-websocket", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.NoError(err)

	wsTrigger := triggerInstance.(*websocket_trigger)

	// Initialize processors
	err = wsTrigger.Start(nil)
	suite.NoError(err)

	// Stop should not panic
	checkpoint, err := wsTrigger.Stop(false)
	suite.NoError(err)
	suite.Nil(checkpoint)
}

func (suite *WebsocketTriggerTestSuite) TestProcessWithWorkerAllocationFailure() {
	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket",
		WorkerAllocatorName: "mock-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr": ":0",
			"is_stream":      false,
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	f := &factory{}
	triggerInstance, err := f.Create(suite.logger, "test-websocket", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.NoError(err)

	wsTrigger := triggerInstance.(*websocket_trigger)

	// Create a failing allocator
	failingAllocator := &mockWorkerAllocator{
		allocateFunc: func(timeout time.Duration) (*worker.Worker, error) {
			return nil, fmt.Errorf("allocation failed")
		},
	}
	wsTrigger.WorkerAllocator = failingAllocator

	// Create a test event
	testEvent := &Event{
		body:      []byte("test message"),
		timestamp: time.Now(),
	}

	// Process should handle allocation failure gracefully
	wsTrigger.process(testEvent) // Should not panic
}

func (suite *WebsocketTriggerTestSuite) TestGetConfig() {
	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket",
		WorkerAllocatorName: "mock-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr": ":8080",
			"buffer_size":    8192,
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	f := &factory{}
	triggerInstance, err := f.Create(suite.logger, "test-websocket", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.NoError(err)

	wsTrigger := triggerInstance.(*websocket_trigger)
	config := wsTrigger.GetConfig()

	suite.NotNil(config)
	suite.Equal(":8080", config["WebSocketAddr"])
	suite.Equal(float64(8192), config["BufferSize"])
}

func TestWebsocketTriggerTestSuite(t *testing.T) {
	suite.Run(t, new(WebsocketTriggerTestSuite))
}

type WebsocketEventTestSuite struct {
	suite.Suite
}

func (suite *WebsocketEventTestSuite) SetupTest() {
}

func (suite *WebsocketEventTestSuite) TestEventMethods() {
	testTime := time.Now()
	attributes := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": 3.14,
	}

	event := &Event{
		body:       []byte("test body"),
		attributes: attributes,
		timestamp:  testTime,
	}

	// Test GetContentType
	suite.Equal("application/octet-stream", event.GetContentType())

	// Test GetBody
	suite.Equal([]byte("test body"), event.GetBody())

	// Test GetHeaderByteSlice
	suite.Equal([]byte("value1"), event.GetHeaderByteSlice("key1"))
	suite.Nil(event.GetHeaderByteSlice("nonexistent"))
	suite.Nil(event.GetHeaderByteSlice("key2")) // int should return nil

	// Test GetHeader
	suite.Equal("value1", event.GetHeader("key1"))
	suite.Equal(42, event.GetHeader("key2"))
	suite.Equal(3.14, event.GetHeader("key3"))
	suite.Nil(event.GetHeader("nonexistent"))

	// Test GetHeaders
	headers := event.GetHeaders()
	suite.Equal(attributes, headers)

	// Test GetHeaderString
	suite.Equal("value1", event.GetHeaderString("key1"))
	suite.Equal("", event.GetHeaderString("nonexistent"))

	// Test GetHeaderInt
	val, err := event.GetHeaderInt("key2")
	suite.NoError(err)
	suite.Equal(42, val)

	val, err = event.GetHeaderInt("key1") // string "value1" should fail
	suite.Error(err)

	val, err = event.GetHeaderInt("nonexistent")
	suite.NoError(err)
	suite.Equal(0, val)

	// Test GetMethod
	suite.Equal("websocket", event.GetMethod())

	// Test GetPath
	suite.Equal("", event.GetPath())

	// Test GetFieldByteSlice (same as GetHeaderByteSlice)
	suite.Equal([]byte("value1"), event.GetFieldByteSlice("key1"))

	// Test GetFieldString (same as GetHeaderString)
	suite.Equal("value1", event.GetFieldString("key1"))

	// Test GetFieldInt (same as GetHeaderInt)
	val, err = event.GetFieldInt("key2")
	suite.NoError(err)
	suite.Equal(42, val)

	// Test GetFields (same as GetHeaders)
	fields := event.GetFields()
	suite.Equal(attributes, fields)

	// Test GetField (same as GetHeader)
	suite.Equal("value1", event.GetField("key1"))

	// Test GetTimestamp
	suite.Equal(testTime, event.GetTimestamp())
}

func (suite *WebsocketEventTestSuite) TestEventWithNilAttributes() {
	event := &Event{
		body:      []byte("test body"),
		timestamp: time.Now(),
	}

	// Test methods with nil attributes
	suite.Nil(event.GetHeader("key"))
	suite.Nil(event.GetHeaders())
	suite.Equal("", event.GetHeaderString("key"))
	val, err := event.GetHeaderInt("key")
	suite.NoError(err)
	suite.Equal(0, val)
}

func TestWebsocketEventTestSuite(t *testing.T) {
	suite.Run(t, new(WebsocketEventTestSuite))
}
