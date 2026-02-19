// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
// SPDX-License-Identifier: Apache-2.0

package sink_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nuclio/zap"
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink"
	_ "github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink/mjpeg"
	_ "github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink/rtsp"
	_ "github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink/webhook"
	_ "github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink/websocket"
	"github.com/stretchr/testify/suite"
)

type SinkIntegrationTestSuite struct {
	suite.Suite
}

// Test MJPEG sink creation and basic operations
func (suite *SinkIntegrationTestSuite) TestMJPEGSinkIntegration() {
	logger, _ := nucliozap.NewNuclioZapTest("test")

	// Create MJPEG sink via registry
	config := map[string]interface{}{
		"port":     19081,
		"path":     "/stream",
		"boundary": "testframe",
	}

	mjpegSink, err := sink.RegistrySingleton.Create(logger, "mjpeg", config)
	suite.NoError(err)
	suite.NotNil(mjpegSink)
	suite.Equal("mjpeg", mjpegSink.GetKind())

	// Start the sink
	err = mjpegSink.Start()
	suite.NoError(err)
	defer mjpegSink.Stop(false)

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Write some frames
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		testFrame := []byte("fake jpeg frame data")
		err = mjpegSink.Write(ctx, testFrame, nil)
		suite.NoError(err)
		time.Sleep(100 * time.Millisecond)
	}

	// Verify configuration
	retrievedConfig := mjpegSink.GetConfig()
	suite.Equal(19081, retrievedConfig["port"])
	suite.Equal("/stream", retrievedConfig["path"])
}

// Test Webhook sink creation and operations
func (suite *SinkIntegrationTestSuite) TestWebhookSinkIntegration() {
	logger, _ := nucliozap.NewNuclioZapTest("test")

	// Create test HTTP server
	receivedCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCount++
		
		// Read and verify body
		body, err := io.ReadAll(r.Body)
		suite.NoError(err)
		suite.Equal("test webhook data", string(body))

		// Check custom header
		suite.Equal("Bearer test-token", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create webhook sink via registry
	config := map[string]interface{}{
		"url":    server.URL,
		"method": "POST",
		"headers": map[string]interface{}{
			"Authorization": "Bearer test-token",
		},
		"timeout":    5,
		"maxRetries": 1,
	}

	webhookSink, err := sink.RegistrySingleton.Create(logger, "webhook", config)
	suite.NoError(err)
	suite.NotNil(webhookSink)
	suite.Equal("webhook", webhookSink.GetKind())

	// Start the sink
	err = webhookSink.Start()
	suite.NoError(err)

	// Write data
	ctx := context.Background()
	err = webhookSink.Write(ctx, []byte("test webhook data"), nil)
	suite.NoError(err)

	// Stop
	err = webhookSink.Stop(false)
	suite.NoError(err)

	// Verify request was received
	suite.Equal(1, receivedCount)
}

// Test WebSocket sink creation and operations
func (suite *SinkIntegrationTestSuite) TestWebSocketSinkIntegration() {
	logger, _ := nucliozap.NewNuclioZapTest("test")

	// Create WebSocket test server
	upgrader := websocket.Upgrader{}
	receivedData := make(chan []byte, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return
		}

		suite.Equal(websocket.BinaryMessage, messageType)
		receivedData <- message
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[4:] // Convert http:// to ws://

	// Create websocket sink via registry
	config := map[string]interface{}{
		"url":         wsURL,
		"messageType": "binary",
		"timeout":     5,
	}

	wsSink, err := sink.RegistrySingleton.Create(logger, "websocket", config)
	suite.NoError(err)
	suite.NotNil(wsSink)
	suite.Equal("websocket", wsSink.GetKind())

	// Start the sink
	err = wsSink.Start()
	suite.NoError(err)
	defer wsSink.Stop(false)

	// Give connection time to establish
	time.Sleep(200 * time.Millisecond)

	// Write data
	ctx := context.Background()
	testData := []byte("test websocket message")
	err = wsSink.Write(ctx, testData, nil)
	suite.NoError(err)

	// Verify data was received
	select {
	case data := <-receivedData:
		suite.Equal(testData, data)
	case <-time.After(2 * time.Second):
		suite.Fail("Timeout waiting for WebSocket message")
	}
}

// Test sink registry functionality
func (suite *SinkIntegrationTestSuite) TestSinkRegistry() {
	// Get registered kinds
	kinds := sink.RegistrySingleton.GetRegisteredKinds()
	suite.Contains(kinds, "mjpeg")
	suite.Contains(kinds, "webhook")
	suite.Contains(kinds, "websocket")
	suite.Contains(kinds, "rtsp")

	// Test getting a factory
	factory, err := sink.RegistrySingleton.Get("mjpeg")
	suite.NoError(err)
	suite.NotNil(factory)
	suite.Equal("mjpeg", factory.GetKind())

	// Test getting non-existent factory
	_, err = sink.RegistrySingleton.Get("nonexistent")
	suite.Error(err)
}

// Test configuration validation
func (suite *SinkIntegrationTestSuite) TestConfigurationValidation() {
	logger, _ := nucliozap.NewNuclioZapTest("test")

	// Test webhook sink - missing URL
	_, err := sink.RegistrySingleton.Create(logger, "webhook", map[string]interface{}{})
	suite.Error(err)
	suite.Contains(err.Error(), "url is required")

	// Test websocket sink - invalid message type
	_, err = sink.RegistrySingleton.Create(logger, "websocket", map[string]interface{}{
		"url":         "ws://example.com",
		"messageType": "invalid",
	})
	suite.Error(err)
	suite.Contains(err.Error(), "invalid message type")

	// Test RTSP sink - invalid type
	_, err = sink.RegistrySingleton.Create(logger, "rtsp", map[string]interface{}{
		"port": 8554,
		"path": "/stream",
		"type": "invalid",
	})
	suite.Error(err)
	suite.Contains(err.Error(), "invalid rtsp type")
}

func TestSinkIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(SinkIntegrationTestSuite))
}
