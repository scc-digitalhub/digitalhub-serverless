/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package mjpeg

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"
)

type mjpeg struct {
	trigger.AbstractTrigger
	configuration *Configuration

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	frameCount int64
}

func newTrigger(logger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {

	abstractTrigger, err := trigger.NewAbstractTrigger(logger,
		workerAllocator,
		&configuration.Configuration,
		"async",
		"mjpeg",
		configuration.Name,
		restartTriggerChan)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	ctx, cancel := context.WithCancel(context.Background())

	newTrigger := mjpeg{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
		ctx:             ctx,
		cancel:          cancel,
		frameCount:      0,
	}
	newTrigger.AbstractTrigger.Trigger = &newTrigger

	return &newTrigger, nil
}

func (m *mjpeg) Start(checkpoint functionconfig.Checkpoint) error {
	m.Logger.DebugWith("Starting MJPEG trigger", "url", m.configuration.URL)

	m.wg.Add(1)
	go m.streamFrames()

	return nil
}

func (m *mjpeg) streamFrames() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			m.Logger.Info("MJPEG trigger stopped")
			return
		default:
			err := m.connectAndStream()
			if err != nil {
				m.Logger.WarnWith("MJPEG stream error, retrying in 5 seconds", "error", err)
				select {
				case <-m.ctx.Done():
					return
				case <-time.After(5 * time.Second):
					continue
				}
			}
		}
	}
}

func (m *mjpeg) connectAndStream() error {
	m.Logger.InfoWith("Connecting to MJPEG stream", "url", m.configuration.URL)

	req, err := http.NewRequestWithContext(m.ctx, "GET", m.configuration.URL, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create request")
	}

	client := &http.Client{
		Timeout: 0, // No timeout for streaming
	}

	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "Failed to connect to MJPEG stream")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	m.Logger.Info("Connected to MJPEG stream, reading frames")

	// Read the boundary from Content-Type header
	boundary := m.extractBoundary(resp.Header.Get("Content-Type"))
	if boundary == "" {
		m.Logger.Warn("Could not extract boundary from Content-Type, using default")
		boundary = "--myboundary"
	} else {
		boundary = "--" + boundary
	}

	return m.readFrames(resp.Body, boundary)
}

func (m *mjpeg) extractBoundary(contentType string) string {
	// Parse Content-Type header to extract boundary
	// Expected format: multipart/x-mixed-replace;boundary=myboundary
	// or with spaces: multipart/x-mixed-replace; boundary = myboundary

	// First, find the "boundary" keyword
	idx := bytes.Index([]byte(contentType), []byte("boundary"))
	if idx == -1 {
		return ""
	}

	// Get the substring starting from "boundary"
	remaining := contentType[idx+len("boundary"):]

	// Find the "=" sign
	eqIdx := bytes.IndexByte([]byte(remaining), '=')
	if eqIdx == -1 {
		return ""
	}

	// Get everything after the "=" and trim spaces
	boundary := bytes.TrimSpace([]byte(remaining[eqIdx+1:]))
	return string(boundary)
}

func (m *mjpeg) readFrames(body io.ReadCloser, boundary string) error {
	reader := bufio.NewReader(body)
	boundaryBytes := []byte(boundary)

	for {
		select {
		case <-m.ctx.Done():
			return nil
		default:
		}

		// Read until boundary
		_, err := m.readUntil(reader, boundaryBytes)
		if err != nil {
			return errors.Wrap(err, "Failed to read boundary")
		}

		// Read headers
		headers, err := m.readHeaders(reader)
		if err != nil {
			return errors.Wrap(err, "Failed to read headers")
		}

		// Get content length
		contentLength := m.getContentLength(headers)
		if contentLength <= 0 {
			m.Logger.Warn("Invalid or missing Content-Length header")
			continue
		}

		// Read frame data
		frameData := make([]byte, contentLength)
		_, err = io.ReadFull(reader, frameData)
		if err != nil {
			return errors.Wrap(err, "Failed to read frame data")
		}

		m.frameCount++

		// Apply processing factor (skip frames if needed)
		if m.frameCount%int64(m.configuration.ProcessingFactor) == 0 {
			m.processFrame(frameData)
		} else {
			m.Logger.DebugWith("Skipping frame", "frame", m.frameCount, "factor", m.configuration.ProcessingFactor)
		}
	}
}

func (m *mjpeg) readUntil(reader *bufio.Reader, delimiter []byte) ([]byte, error) {
	var result []byte
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		result = append(result, line...)
		if bytes.Contains(line, delimiter) {
			return result, nil
		}
	}
}

func (m *mjpeg) readHeaders(reader *bufio.Reader) (map[string]string, error) {
	headers := make(map[string]string)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}

		// Empty line marks end of headers
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			break
		}

		// Parse header
		parts := bytes.SplitN(trimmed, []byte(":"), 2)
		if len(parts) == 2 {
			key := string(bytes.TrimSpace(parts[0]))
			value := string(bytes.TrimSpace(parts[1]))
			headers[key] = value
		}
	}
	return headers, nil
}

func (m *mjpeg) getContentLength(headers map[string]string) int {
	// Try different case variations
	for _, key := range []string{"Content-Length", "content-length", "Content-length"} {
		if val, ok := headers[key]; ok {
			length, err := strconv.Atoi(val)
			if err == nil && length > 0 {
				return length
			}
		}
	}
	return 0
}

func (m *mjpeg) processFrame(frameData []byte) {
	event := &Event{
		body:      frameData,
		timestamp: time.Now(),
		frameNum:  m.frameCount,
		url:       m.configuration.URL,
	}

	m.Logger.DebugWith("Processing frame",
		"frame", m.frameCount,
		"size", len(frameData))

	// Allocate worker and submit event
	response, submitError, processError := m.AllocateWorkerAndSubmitEvent(
		event,
		m.Logger,
		10*time.Second)

	if submitError != nil {
		m.Logger.WarnWith("Failed to submit frame event", "error", submitError)
		return
	}

	if processError != nil {
		m.Logger.WarnWith("Failed to process frame event", "error", processError)
		return
	}

	m.Logger.DebugWith("Frame processed successfully",
		"frame", m.frameCount,
		"response", response)
}

func (m *mjpeg) Stop(force bool) (functionconfig.Checkpoint, error) {
	m.Logger.Info("Stopping MJPEG trigger")

	m.cancel()

	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.Logger.Info("MJPEG trigger stopped successfully")
	case <-time.After(10 * time.Second):
		m.Logger.Warn("Timeout waiting for MJPEG trigger to stop")
	}

	return nil, nil
}

func (m *mjpeg) GetConfig() map[string]interface{} {
	return common.StructureToMap(m.configuration)
}
