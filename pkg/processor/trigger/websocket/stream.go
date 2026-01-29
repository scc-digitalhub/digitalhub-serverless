/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package websocket

import (
	"sync"
	"time"
)

// DataProcessorStream aggregates incoming byte stream into fixed-size chunks,
// keeps a rolling buffer, and periodically emits snapshots as Events.
type DataProcessorStream struct {
	lock       sync.Mutex
	chunkBytes int
	maxBytes   int
	trimBytes  int
	chunkBuf   []byte
	buffer     []byte
	newBytes   int
	output     chan *Event
	stop       chan struct{}
}

func NewDataProcessorStream(
	chunkBytes,
	maxBytes,
	trimBytes int,
) *DataProcessorStream {
	return &DataProcessorStream{
		chunkBytes: chunkBytes,
		maxBytes:   maxBytes,
		trimBytes:  trimBytes,
		chunkBuf:   []byte{},
		buffer:     []byte{},
		output:     make(chan *Event, 8),
		stop:       make(chan struct{}),
	}
}

func (dp *DataProcessorStream) Start(processingInterval time.Duration) {
	go dp.loop(processingInterval)
}

func (dp *DataProcessorStream) Stop() {
	close(dp.stop)
}

// append raw incoming data and convert it into fixed-size chunks.
// Chunks are appended to rolling buffer.
func (dp *DataProcessorStream) Push(data []byte) {
	dp.lock.Lock()
	defer dp.lock.Unlock()

	dp.chunkBuf = append(dp.chunkBuf, data...)

	for len(dp.chunkBuf) >= dp.chunkBytes {
		chunk := make([]byte, dp.chunkBytes)
		copy(chunk, dp.chunkBuf[:dp.chunkBytes])
		dp.chunkBuf = dp.chunkBuf[dp.chunkBytes:]
		dp.buffer = append(dp.buffer, chunk...)
		if len(dp.buffer) > dp.maxBytes {
			dp.buffer = dp.buffer[dp.trimBytes:]
		}
		dp.newBytes += len(chunk)
	}
}

func (dp *DataProcessorStream) loop(processingInterval time.Duration) {
	ticker := time.NewTicker(processingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-dp.stop:
			return
		case <-ticker.C:
			if ev := dp.tryEmit(); ev != nil {
				dp.output <- ev
			}
		}
	}
}

// emit an event only if enough new data has arrived
// since last emission (at least one full chunk)
func (dp *DataProcessorStream) tryEmit() *Event {
	dp.lock.Lock()
	defer dp.lock.Unlock()

	if dp.newBytes < dp.chunkBytes {
		return nil
	}

	snapshot := make([]byte, len(dp.buffer))
	copy(snapshot, dp.buffer)

	dp.newBytes = 0

	return &Event{
		body:      snapshot,
		timestamp: time.Now(),
	}
}

func (dp *DataProcessorStream) Output() <-chan *Event {
	return dp.output
}
