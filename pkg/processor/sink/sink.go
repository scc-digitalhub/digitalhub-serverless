// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
// SPDX-License-Identifier: Apache-2.0

package sink

import (
	"context"

	"github.com/nuclio/logger"
)

// Sink defines the interface for streaming worker outputs
type Sink interface {
	// Start initializes and starts the sink
	Start() error

	// Stop gracefully shuts down the sink
	Stop(force bool) error

	// Write sends data to the sink
	Write(ctx context.Context, data []byte, metadata map[string]interface{}) error

	// GetKind returns the sink type (e.g., "rtsp", "mjpeg", "websocket", "webhook")
	GetKind() string

	// GetConfig returns the sink configuration
	GetConfig() map[string]interface{}
}

// Factory creates sinks
type Factory interface {
	// Create creates a new sink instance
	Create(logger logger.Logger, configuration map[string]interface{}) (Sink, error)

	// GetKind returns the kind of sink this factory creates
	GetKind() string
}
