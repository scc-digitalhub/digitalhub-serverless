/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
*/

package rtsp

import (
	"encoding/binary"
	"time"
)

// AudioProcessor processes audio data, accumulating bytes into fixed-size chunks
type AudioProcessor struct {
	*DataProcessor
	buffer []byte
}

func NewAudioProcessor(chunkBytes, maxBytes, trimBytes int) *AudioProcessor {
	return &AudioProcessor{
		&DataProcessor{
			chunkBytes: chunkBytes,
			maxBytes:   maxBytes,
			trimBytes:  trimBytes,
			chunkBuf:   []byte{},
			output:     make(chan *Event, 8),
			stop:       make(chan struct{}),
		},
		[]byte{},
	}
}

func (ap *AudioProcessor) Push(data any) {
	ap.lock.Lock()
	defer ap.lock.Unlock()

	b, ok := data.([]byte)
	if !ok {
		return
	}
	ap.chunkBuf = append(ap.chunkBuf, b...)

	for len(ap.chunkBuf) >= ap.chunkBytes {
		chunk := make([]byte, ap.chunkBytes)
		copy(chunk, ap.chunkBuf[:ap.chunkBytes])
		ap.chunkBuf = ap.chunkBuf[ap.chunkBytes:]

		ap.buffer = append(ap.buffer, chunk...)

		if len(ap.buffer) > ap.maxBytes {
			ap.buffer = ap.buffer[ap.trimBytes:]
		}

		ap.newBytes += len(chunk)
	}
}

func (ap *AudioProcessor) tryEmit() *Event {
	ap.lock.Lock()
	defer ap.lock.Unlock()

	if ap.newBytes < ap.chunkBytes {
		return nil
	}

	snapshot := make([]byte, len(ap.buffer))
	copy(snapshot, ap.buffer)
	ap.newBytes = 0

	return &Event{
		body:      snapshot,
		timestamp: time.Now(),
	}
}

func (ap *AudioProcessor) Start(interval time.Duration) {
	go ap.audioLoop(interval)
}

func (ap *AudioProcessor) audioLoop(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ap.stop:
			return
		case <-t.C:
			if ev := ap.tryEmit(); ev != nil {
				ap.output <- ev
			}
		}
	}
}

func convertBigEndianToLittleEndian(in []byte) []byte {
	out := make([]byte, len(in))
	for i := 0; i+1 < len(in); i += 2 {
		v := binary.BigEndian.Uint16(in[i:])
		binary.LittleEndian.PutUint16(out[i:], v)
	}
	return out
}
