// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
// SPDX-License-Identifier: Apache-2.0

package mjpeg

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/logger"
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink"
)

// Configuration for MJPEG sink
type Configuration struct {
	Port     int    `json:"port,omitempty"`
	Path     string `json:"path,omitempty"`
	Boundary string `json:"boundary,omitempty"`
}

// Sink implements MJPEG HTTP streaming
type Sink struct {
	logger        logger.Logger
	configuration *Configuration
	server        *http.Server
	frameChan     chan []byte
	clients       map[chan []byte]struct{}
	clientsMux    sync.RWMutex
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// factory implements sink.Factory
type factory struct{}

func (f *factory) Create(logger logger.Logger, configuration map[string]interface{}) (sink.Sink, error) {
	config := &Configuration{
		Port:     8081,
		Path:     "/stream",
		Boundary: "frame",
	}

	if err := mapstructure.Decode(configuration, config); err != nil {
		return nil, fmt.Errorf("failed to parse mjpeg sink configuration: %w", err)
	}

	return &Sink{
		logger:        logger,
		configuration: config,
		frameChan:     make(chan []byte, 10),
		clients:       make(map[chan []byte]struct{}),
		stopChan:      make(chan struct{}),
	}, nil
}

func (f *factory) GetKind() string {
	return "mjpeg"
}

// Start starts the MJPEG HTTP server
func (s *Sink) Start() error {
	s.logger.InfoWith("Starting MJPEG sink", "port", s.configuration.Port, "path", s.configuration.Path)

	mux := http.NewServeMux()
	mux.HandleFunc(s.configuration.Path, s.handleStream)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.configuration.Port),
		Handler: mux,
	}

	// Start frame broadcaster
	s.wg.Add(1)
	go s.broadcastFrames()

	// Start HTTP server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.ErrorWith("MJPEG server error", "error", err)
		}
	}()

	return nil
}

// Stop stops the MJPEG sink
func (s *Sink) Stop(force bool) error {
	s.logger.InfoWith("Stopping MJPEG sink", "force", force)

	close(s.stopChan)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.WarnWith("MJPEG server shutdown error", "error", err)
	}

	s.wg.Wait()

	close(s.frameChan)

	s.clientsMux.Lock()
	for clientChan := range s.clients {
		close(clientChan)
	}
	s.clients = make(map[chan []byte]struct{})
	s.clientsMux.Unlock()

	return nil
}

// Write sends a frame to the MJPEG stream
func (s *Sink) Write(ctx context.Context, data []byte, metadata map[string]interface{}) error {
	select {
	case s.frameChan <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		// Drop frame if channel is full
		s.logger.DebugWith("Dropping frame - channel full")
		return nil
	}
}

// GetKind returns the sink type
func (s *Sink) GetKind() string {
	return "mjpeg"
}

// GetConfig returns the sink configuration
func (s *Sink) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"port":     s.configuration.Port,
		"path":     s.configuration.Path,
		"boundary": s.configuration.Boundary,
	}
}

// handleStream handles HTTP requests for the MJPEG stream
func (s *Sink) handleStream(w http.ResponseWriter, r *http.Request) {
	s.logger.DebugWith("New MJPEG client connected", "remote", r.RemoteAddr)

	w.Header().Set("Content-Type", fmt.Sprintf("multipart/x-mixed-replace; boundary=%s", s.configuration.Boundary))
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "close")

	clientChan := make(chan []byte, 5)

	s.clientsMux.Lock()
	s.clients[clientChan] = struct{}{}
	s.clientsMux.Unlock()

	defer func() {
		s.clientsMux.Lock()
		delete(s.clients, clientChan)
		s.clientsMux.Unlock()
		close(clientChan)
		s.logger.DebugWith("MJPEG client disconnected", "remote", r.RemoteAddr)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.logger.ErrorWith("Response writer does not support flushing")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case frame, ok := <-clientChan:
			if !ok {
				return
			}

			// Write boundary
			if _, err := fmt.Fprintf(w, "--%s\r\n", s.configuration.Boundary); err != nil {
				return
			}

			// Write headers
			if _, err := fmt.Fprintf(w, "Content-Type: image/jpeg\r\n"); err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(frame)); err != nil {
				return
			}

			// Write frame data
			if _, err := w.Write(frame); err != nil {
				return
			}

			// Write trailing newline
			if _, err := w.Write([]byte("\r\n")); err != nil {
				return
			}

			flusher.Flush()

		case <-r.Context().Done():
			return

		case <-s.stopChan:
			return
		}
	}
}

// broadcastFrames broadcasts frames to all connected clients
func (s *Sink) broadcastFrames() {
	defer s.wg.Done()

	for {
		select {
		case frame, ok := <-s.frameChan:
			if !ok {
				return
			}

			s.clientsMux.RLock()
			for clientChan := range s.clients {
				select {
				case clientChan <- frame:
					// Frame sent successfully
				default:
					// Client channel full - drop frame for this client
					s.logger.DebugWith("Dropping frame for slow client")
				}
			}
			s.clientsMux.RUnlock()

		case <-s.stopChan:
			return
		}
	}
}

func init() {
	sink.RegistrySingleton.Register("mjpeg", &factory{})
}
