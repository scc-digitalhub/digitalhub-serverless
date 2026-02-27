// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/logger"
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink"
)

// Configuration for WebSocket sink
type Configuration struct {
	URL         string `json:"url"`
	MessageType string `json:"messageType,omitempty"` // "text" or "binary"
	Timeout     int    `json:"timeout,omitempty"`     // seconds
}

// Sink implements WebSocket client
type Sink struct {
	logger        logger.Logger
	configuration *Configuration
	conn          *websocket.Conn
	connMux       sync.RWMutex
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// factory implements sink.Factory
type factory struct{}

func (f *factory) Create(logger logger.Logger, configuration map[string]interface{}) (sink.Sink, error) {
	config := &Configuration{
		MessageType: "binary",
		Timeout:     10,
	}

	if err := mapstructure.Decode(configuration, config); err != nil {
		return nil, fmt.Errorf("failed to parse websocket sink configuration: %w", err)
	}

	if config.URL == "" {
		return nil, fmt.Errorf("websocket url is required")
	}

	if config.MessageType != "text" && config.MessageType != "binary" {
		return nil, fmt.Errorf("invalid message type: %s (must be 'text' or 'binary')", config.MessageType)
	}

	return &Sink{
		logger:        logger,
		configuration: config,
		stopChan:      make(chan struct{}),
	}, nil
}

func (f *factory) GetKind() string {
	return "websocket"
}

// Start starts the WebSocket connection
func (s *Sink) Start() error {
	s.logger.InfoWith("Starting WebSocket sink", "url", s.configuration.URL)

	if err := s.connect(); err != nil {
		return err
	}

	// Start connection manager
	s.wg.Add(1)
	go s.manageConnection()

	return nil
}

// Stop stops the WebSocket sink
func (s *Sink) Stop(force bool) error {
	s.logger.InfoWith("Stopping WebSocket sink", "force", force)

	close(s.stopChan)

	s.connMux.Lock()
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	s.connMux.Unlock()

	s.wg.Wait()

	return nil
}

// Write sends data to the WebSocket
func (s *Sink) Write(ctx context.Context, data []byte, metadata map[string]interface{}) error {
	s.connMux.RLock()
	conn := s.conn
	s.connMux.RUnlock()

	if conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	messageType := websocket.BinaryMessage
	if s.configuration.MessageType == "text" {
		messageType = websocket.TextMessage
	}

	if err := conn.WriteMessage(messageType, data); err != nil {
		s.logger.WarnWith("Failed to write to websocket", "error", err)
		// Trigger reconnection
		s.connMux.Lock()
		if s.conn != nil {
			s.conn.Close()
			s.conn = nil
		}
		s.connMux.Unlock()
		return err
	}

	return nil
}

// GetKind returns the sink type
func (s *Sink) GetKind() string {
	return "websocket"
}

// GetConfig returns the sink configuration
func (s *Sink) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"url":         s.configuration.URL,
		"messageType": s.configuration.MessageType,
		"timeout":     s.configuration.Timeout,
	}
}

// connect establishes a WebSocket connection
func (s *Sink) connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: time.Duration(s.configuration.Timeout) * time.Second,
	}

	conn, _, err := dialer.Dial(s.configuration.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to websocket: %w", err)
	}

	s.connMux.Lock()
	s.conn = conn
	s.connMux.Unlock()

	return nil
}

// manageConnection monitors and reconnects the WebSocket
func (s *Sink) manageConnection() {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.connMux.RLock()
			connected := s.conn != nil
			s.connMux.RUnlock()

			if !connected {
				s.logger.DebugWith("Attempting to reconnect WebSocket")
				if err := s.connect(); err != nil {
					s.logger.WarnWith("Failed to reconnect", "error", err)
				} else {
					s.logger.InfoWith("WebSocket reconnected")
				}
			}

		case <-s.stopChan:
			return
		}
	}
}

func init() {
	sink.RegistrySingleton.Register("websocket", &factory{})
}
