// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/logger"
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink"
)

// Configuration for Webhook sink
type Configuration struct {
	URL        string            `json:"url"`
	Method     string            `json:"method,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Timeout    int               `json:"timeout,omitempty"`    // seconds
	MaxRetries int               `json:"maxRetries,omitempty"` // max retry attempts
	RetryDelay int               `json:"retryDelay,omitempty"` // seconds between retries
}

// Sink implements HTTP webhook client
type Sink struct {
	logger        logger.Logger
	configuration *Configuration
	client        *http.Client
}

// factory implements sink.Factory
type factory struct{}

func (f *factory) Create(logger logger.Logger, configuration map[string]interface{}) (sink.Sink, error) {
	config := &Configuration{
		Method:     "POST",
		Timeout:    10,
		MaxRetries: 3,
		RetryDelay: 1,
		Headers:    make(map[string]string),
	}

	if err := mapstructure.Decode(configuration, config); err != nil {
		return nil, fmt.Errorf("failed to parse webhook sink configuration: %w", err)
	}

	if config.URL == "" {
		return nil, fmt.Errorf("webhook url is required")
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	return &Sink{
		logger:        logger,
		configuration: config,
		client:        client,
	}, nil
}

func (f *factory) GetKind() string {
	return "webhook"
}

// Start starts the webhook sink (no-op for webhook)
func (s *Sink) Start() error {
	s.logger.InfoWith("Starting Webhook sink", "url", s.configuration.URL)
	return nil
}

// Stop stops the webhook sink (no-op for webhook)
func (s *Sink) Stop(force bool) error {
	s.logger.InfoWith("Stopping Webhook sink", "force", force)
	return nil
}

// Write sends data to the webhook
func (s *Sink) Write(ctx context.Context, data []byte, metadata map[string]interface{}) error {
	var lastErr error

	for attempt := 0; attempt <= s.configuration.MaxRetries; attempt++ {
		if attempt > 0 {
			s.logger.DebugWith("Retrying webhook request", "attempt", attempt)
			time.Sleep(time.Duration(s.configuration.RetryDelay) * time.Second)
		}

		req, err := http.NewRequestWithContext(ctx, s.configuration.Method, s.configuration.URL, bytes.NewReader(data))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		// Set headers
		for key, value := range s.configuration.Headers {
			req.Header.Set(key, value)
		}

		// Default content type if not specified
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/octet-stream")
		}

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to send request: %w", err)
			continue
		}

		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return fmt.Errorf("webhook failed after %d attempts: %w", s.configuration.MaxRetries+1, lastErr)
}

// GetKind returns the sink type
func (s *Sink) GetKind() string {
	return "webhook"
}

// GetConfig returns the sink configuration
func (s *Sink) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"url":        s.configuration.URL,
		"method":     s.configuration.Method,
		"headers":    s.configuration.Headers,
		"timeout":    s.configuration.Timeout,
		"maxRetries": s.configuration.MaxRetries,
		"retryDelay": s.configuration.RetryDelay,
	}
}

func init() {
	sink.RegistrySingleton.Register("webhook", &factory{})
}
