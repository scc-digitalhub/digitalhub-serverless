package websocket

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type WebsocketIntegrationTestSuite struct {
	suite.Suite
	logger                logger.Logger
	namedWorkerAllocators *worker.AllocatorSyncMap
	serverAddr            string
}

func (suite *WebsocketIntegrationTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("integration-test")
	suite.Require().NoError(err)

	suite.namedWorkerAllocators = worker.NewAllocatorSyncMap()
	allocator, _ := newMockWorkerAllocator()
	suite.namedWorkerAllocators.Store("mock-allocator", allocator)

	// Use a fixed port for testing to avoid address resolution issues
	suite.serverAddr = ":18080"
}

func (suite *WebsocketIntegrationTestSuite) TearDownTest() {
	// Cleanup if needed
}

func (suite *WebsocketIntegrationTestSuite) TestDiscreteModeIntegration() {
	// Create trigger configuration for discrete mode
	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket-discrete",
		WorkerAllocatorName: "mock-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr":      suite.serverAddr,
			"processing_interval": 100, // Fast processing for tests
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
	triggerInstance, err := f.Create(suite.logger, "test-websocket-discrete", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.NoError(err)
	suite.NotNil(triggerInstance)

	wsTrigger := triggerInstance.(*websocket_trigger)

	// Start the trigger
	err = wsTrigger.Start(nil)
	suite.NoError(err)

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Connect websocket client using the fixed port
	u := url.URL{Scheme: "ws", Host: "localhost" + suite.serverAddr, Path: "/ws"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	suite.NoError(err)
	defer conn.Close()

	// Test sending a message
	testMessage := "Hello WebSocket"
	err = conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
	suite.NoError(err)

	// Wait for processing interval
	time.Sleep(200 * time.Millisecond)

	// The mock worker doesn't send responses, so we just verify the connection works
	// In a real scenario, we'd check for response messages

	// Test connection cleanup
	conn.Close()

	// Stop the trigger
	_, err = wsTrigger.Stop(false)
	suite.NoError(err)
}

func (suite *WebsocketIntegrationTestSuite) TestStreamModeIntegration() {
	// Create trigger configuration for stream mode
	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket-stream",
		WorkerAllocatorName: "mock-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr":      suite.serverAddr,
			"chunk_bytes":         10,  // Small chunks for testing
			"max_bytes":           100, // Small buffer for testing
			"trim_bytes":          50,  // Trim half
			"processing_interval": 50,  // Fast processing
			"is_stream":           true,
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
	triggerInstance, err := f.Create(suite.logger, "test-websocket-stream", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.NoError(err)
	suite.NotNil(triggerInstance)

	wsTrigger := triggerInstance.(*websocket_trigger)

	// Start the trigger
	err = wsTrigger.Start(nil)
	suite.NoError(err)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect websocket client using the fixed port
	u := url.URL{Scheme: "ws", Host: "localhost" + suite.serverAddr, Path: "/ws"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	suite.NoError(err)
	defer conn.Close()

	// Send data in chunks smaller than chunk_bytes to test accumulation
	data := []byte("abcdefghij") // 10 bytes = exactly 1 chunk
	err = conn.WriteMessage(websocket.BinaryMessage, data)
	suite.NoError(err)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Send more data to test buffer management
	data2 := []byte("klmnopqrst") // Another 10 bytes
	err = conn.WriteMessage(websocket.BinaryMessage, data2)
	suite.NoError(err)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Test connection cleanup
	conn.Close()

	// Stop the trigger
	_, err = wsTrigger.Stop(false)
	suite.NoError(err)
}

func (suite *WebsocketIntegrationTestSuite) TestConcurrentConnections() {
	// Create trigger configuration
	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket-concurrent",
		WorkerAllocatorName: "mock-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr":      suite.serverAddr,
			"processing_interval": 100,
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
	triggerInstance, err := f.Create(suite.logger, "test-websocket-concurrent", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.NoError(err)
	suite.NotNil(triggerInstance)

	wsTrigger := triggerInstance.(*websocket_trigger)

	// Start the trigger
	err = wsTrigger.Start(nil)
	suite.NoError(err)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test multiple concurrent connections
	numConnections := 3
	var wg sync.WaitGroup
	connections := make([]*websocket.Conn, numConnections)

	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			u := url.URL{Scheme: "ws", Host: "localhost" + suite.serverAddr, Path: "/ws"}
			conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				suite.T().Logf("Failed to connect client %d: %v", index, err)
				return
			}

			connections[index] = conn

			// Send a message
			message := fmt.Sprintf("Message from client %d", index)
			err = conn.WriteMessage(websocket.TextMessage, []byte(message))
			if err != nil {
				suite.T().Logf("Failed to send message from client %d: %v", index, err)
				conn.Close()
				return
			}

			// Keep connection open briefly
			time.Sleep(50 * time.Millisecond)

			conn.Close()
		}(i)
	}

	wg.Wait()

	// Give time for connections to be cleaned up
	time.Sleep(100 * time.Millisecond)

	// Verify all connections were handled
	suite.Equal(0, len(wsTrigger.conns)) // All should be cleaned up

	// Stop the trigger
	_, err = wsTrigger.Stop(false)
	suite.NoError(err)
}

func (suite *WebsocketIntegrationTestSuite) TestConnectionLimit() {
	// Create trigger with limited connections (1 worker = 1 max client)
	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket-limit",
		WorkerAllocatorName: "mock-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr":      suite.serverAddr,
			"processing_interval": 100,
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
	triggerInstance, err := f.Create(suite.logger, "test-websocket-limit", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.NoError(err)
	suite.NotNil(triggerInstance)

	wsTrigger := triggerInstance.(*websocket_trigger)

	// Start the trigger
	err = wsTrigger.Start(nil)
	suite.NoError(err)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// First connection should succeed
	u := url.URL{Scheme: "ws", Host: "localhost" + suite.serverAddr, Path: "/ws"}
	conn1, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	suite.NoError(err)
	defer conn1.Close()

	// Give time for connection to be established
	time.Sleep(50 * time.Millisecond)

	// Second connection should be rejected (only 1 worker/client allowed)
	conn2, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err == nil {
		conn2.Close()
		suite.Fail("Expected second connection to be rejected")
	} else {
		suite.T().Logf("Second connection correctly rejected: %v", err)
	}

	// Close first connection
	conn1.Close()

	// Stop the trigger
	_, err = wsTrigger.Stop(false)
	suite.NoError(err)
}

func (suite *WebsocketIntegrationTestSuite) TestHTTPRequestsRejected() {
	// Create trigger configuration
	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket-http",
		WorkerAllocatorName: "mock-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr":      suite.serverAddr,
			"processing_interval": 100,
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
	triggerInstance, err := f.Create(suite.logger, "test-websocket-http", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.NoError(err)
	suite.NotNil(triggerInstance)

	wsTrigger := triggerInstance.(*websocket_trigger)

	// Start the trigger
	err = wsTrigger.Start(nil)
	suite.NoError(err)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test regular HTTP request (should fail websocket upgrade)
	serverAddr := "localhost" + suite.serverAddr
	resp, err := http.Get("http://" + serverAddr + "/ws")
	suite.NoError(err)
	defer resp.Body.Close()

	// Should get 400 Bad Request (websocket upgrade required)
	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	// Stop the trigger
	_, err = wsTrigger.Stop(false)
	suite.NoError(err)
}

func TestWebsocketIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(WebsocketIntegrationTestSuite))
}
