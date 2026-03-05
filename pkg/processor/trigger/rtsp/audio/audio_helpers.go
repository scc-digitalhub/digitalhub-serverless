/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
*/

package audio

import (
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/trigger/rtsp/helpers"
)

// AudioProcessor processes audio data, accumulating bytes into fixed-size chunks
// It extends BaseMediaProcessor with audio-specific buffer accumulation logic
type AudioProcessor struct {
	*helpers.BaseMediaProcessor
	buffer []byte
}

// NewAudioProcessor creates a new audio processor with specified buffer parameters
func NewAudioProcessor(chunkBytes, maxBytes, trimBytes int) *AudioProcessor {
	return &AudioProcessor{
		BaseMediaProcessor: helpers.NewBaseMediaProcessor(chunkBytes, maxBytes, trimBytes),
		buffer:             []byte{},
	}
}

// Push adds audio data to be processed, accumulating into fixed-size chunks
func (ap *AudioProcessor) Push(data any) {
	ap.Lock.Lock()
	defer ap.Lock.Unlock()

	b, ok := data.([]byte)
	if !ok {
		return
	}
	ap.ChunkBuf = append(ap.ChunkBuf, b...)

	// Accumulate chunks until we have enough bytes
	for len(ap.ChunkBuf) >= ap.ChunkBytes {
		chunk := make([]byte, ap.ChunkBytes)
		copy(chunk, ap.ChunkBuf[:ap.ChunkBytes])
		ap.ChunkBuf = ap.ChunkBuf[ap.ChunkBytes:]

		ap.buffer = append(ap.buffer, chunk...)

		// Trim buffer if it exceeds maximum size
		if len(ap.buffer) > ap.MaxBytes {
			ap.buffer = ap.buffer[ap.TrimBytes:]
		}

		ap.NewBytes += len(chunk)
	}
}

// tryEmit checks if we have enough data to emit and returns an event if ready
func (ap *AudioProcessor) tryEmit() *helpers.Event {
	ap.Lock.Lock()
	defer ap.Lock.Unlock()

	if ap.NewBytes < ap.ChunkBytes {
		return nil
	}

	// Create a snapshot of the current buffer
	snapshot := make([]byte, len(ap.buffer))
	copy(snapshot, ap.buffer)
	ap.NewBytes = 0

	return &helpers.Event{
		Body:      snapshot,
		Timestamp: time.Now(),
	}
}

// Start begins audio processing with the specified interval
func (ap *AudioProcessor) Start(interval time.Duration) {
	ap.BaseStart(interval, ap.eventLoop)
}

// eventLoop is the main processing loop for audio data
func (ap *AudioProcessor) eventLoop(interval time.Duration) {
	ap.CommonEventLoop(interval, ap.tryEmit)
}

// ProcessRTP handles audio RTP packet depacketization
func (ap *AudioProcessor) ProcessRTP(pkt *rtp.Packet, forma format.Format) (any, error) {
	pipeline := ap.BaseMediaProcessor.GetPipeline()
	if pipeline == nil {
		return pkt.Payload, nil
	}

	mp, ok := pipeline.(*helpers.MediaPipeline)
	if !ok {
		return pkt.Payload, nil
	}

	return mp.ProcessRTP(pkt, forma)
}
