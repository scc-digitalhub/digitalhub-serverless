/*
SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
*/

package helpers

import (
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

// MediaProcessor defines the interface for processing media data (audio or video)
// It handles both RTP packet depacketization and buffered event emission
type MediaProcessor interface {
	// Start begins processing with specified interval
	Start(interval time.Duration)
	// Stop signals the processor to stop
	Stop()
	// ProcessRTP decodes an RTP packet based on format type
	ProcessRTP(pkt *rtp.Packet, forma format.Format) (any, error)
	// Push adds data to be processed
	Push(data any)
	// Output returns the channel for processed events
	Output() <-chan *Event
	// SetPipeline sets the underlying media pipeline processor
	SetPipeline(p any)
}

// BaseMediaProcessor is the abstract base for all media processors
// It provides common fields and functionality for accumulating and emitting data
type BaseMediaProcessor struct {
	Lock          sync.Mutex
	ChunkBytes    int
	MaxBytes      int
	TrimBytes     int
	ChunkBuf      []byte
	NewBytes      int
	OutputChannel chan *Event
	StopChannel   chan struct{}
	pipeline      any // holds the underlying media pipeline processor
}

// BaseStart starts the processor's event loop
// Subclasses should override the loop method for specific behavior
func (bmp *BaseMediaProcessor) BaseStart(interval time.Duration, loopFunc func(time.Duration)) {
	go loopFunc(interval)
}

// Output returns the output channel for processed events
func (bmp *BaseMediaProcessor) Output() <-chan *Event {
	return bmp.OutputChannel
}

// Stop signals the processor to stop processing
func (bmp *BaseMediaProcessor) Stop() {
	close(bmp.StopChannel)
}

// CommonEventLoop provides the standard event loop behavior
// Used by both audio and video processors
func (bmp *BaseMediaProcessor) CommonEventLoop(interval time.Duration, emitter func() *Event) {
	t := time.NewTicker(interval)
	defer t.Stop()
	defer close(bmp.OutputChannel)

	for {
		select {
		case <-bmp.StopChannel:
			return
		case <-t.C:
			if ev := emitter(); ev != nil {
				bmp.OutputChannel <- ev
			}
		}
	}
}

// NewBaseMediaProcessor creates a new base media processor with default channels
func NewBaseMediaProcessor(chunkBytes, maxBytes, trimBytes int) *BaseMediaProcessor {
	return &BaseMediaProcessor{
		ChunkBytes:    chunkBytes,
		MaxBytes:      maxBytes,
		TrimBytes:     trimBytes,
		ChunkBuf:      []byte{},
		OutputChannel: make(chan *Event, 8),
		StopChannel:   make(chan struct{}),
		pipeline:      nil,
	}
}

// SetPipeline sets the underlying media pipeline processor
func (bmp *BaseMediaProcessor) SetPipeline(p any) {
	bmp.pipeline = p
}

// GetPipeline returns the underlying media pipeline processor
func (bmp *BaseMediaProcessor) GetPipeline() any {
	return bmp.pipeline
}
