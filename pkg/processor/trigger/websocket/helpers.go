/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package websocket

import (
	"sync"
)

// AudioProcessor accumulates PCM audio into chunks and manages a rolling buffer
type AudioProcessor struct {
	lock             sync.Mutex
	chunkBytes       int
	maxBytes         int
	trimBytes        int
	chunkBuf         []byte
	buffer           []byte
	accumulateBuffer bool
}

// NewAudioProcessor creates a new audio processor
func NewAudioProcessor(sampleRate, chunkDurationSeconds, maxBufferSeconds, trimSeconds int, accumulateBuffer bool) *AudioProcessor {
	return &AudioProcessor{
		chunkBytes:       chunkDurationSeconds * sampleRate * 2, // 2 bytes per sample (16-bit)
		maxBytes:         maxBufferSeconds * sampleRate * 2,
		trimBytes:        trimSeconds * sampleRate * 2,
		chunkBuf:         []byte{},
		buffer:           []byte{},
		accumulateBuffer: accumulateBuffer,
	}
}

// AddPCM adds PCM data to the processor and returns any complete chunks
// Returns a slice of complete chunks (each as []byte)
func (ap *AudioProcessor) AddPCM(pcm []byte) [][]byte {
	ap.lock.Lock()
	defer ap.lock.Unlock()

	var chunks [][]byte

	// Append incoming data to temp buffer
	ap.chunkBuf = append(ap.chunkBuf, pcm...)

	// Extract complete chunks
	for len(ap.chunkBuf) >= ap.chunkBytes {
		// Get one complete chunk
		chunk := make([]byte, ap.chunkBytes)
		copy(chunk, ap.chunkBuf[:ap.chunkBytes])
		ap.chunkBuf = ap.chunkBuf[ap.chunkBytes:]

		if ap.accumulateBuffer {
			// Add chunk to rolling buffer
			ap.buffer = append(ap.buffer, chunk...)

			// Trim buffer if it exceeds max size
			if len(ap.buffer) > ap.maxBytes {
				ap.buffer = ap.buffer[ap.trimBytes:]
			}
		} else {
			ap.buffer = chunk
		}

		// Add chunk to output
		chunks = append(chunks, chunk)
	}

	return chunks
}
