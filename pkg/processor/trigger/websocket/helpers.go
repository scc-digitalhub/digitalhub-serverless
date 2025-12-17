/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package websocket

import (
	"sync"
	"time"
)

type DataProcessor struct {
	lock sync.Mutex

	chunkBytes int
	maxBytes   int
	trimBytes  int
	sleepTime  time.Duration
	isStream   bool

	chunkBuf []byte
	buffer   []byte

	newBytes int // bytes added since last emit

	output chan *Event
	stop   chan struct{}
}

func NewDataProcessor(
	chunkBytes,
	maxBytes,
	trimBytes int,
	sleepTime time.Duration,
	isStream bool,
) *DataProcessor {

	return &DataProcessor{
		chunkBytes: chunkBytes,
		maxBytes:   maxBytes,
		trimBytes:  trimBytes,
		sleepTime:  sleepTime,
		isStream:   isStream,
		chunkBuf:   []byte{},
		buffer:     []byte{},
		output:     make(chan *Event, 8),
		stop:       make(chan struct{}),
	}
}

func (dp *DataProcessor) Start() {
	go dp.loop()
}

func (dp *DataProcessor) Stop() {
	close(dp.stop)
}

func (dp *DataProcessor) manageBuffer(data []byte) {
	dp.lock.Lock()
	defer dp.lock.Unlock()

	dp.chunkBuf = append(dp.chunkBuf, data...)

	for len(dp.chunkBuf) >= dp.chunkBytes {
		chunk := make([]byte, dp.chunkBytes)
		copy(chunk, dp.chunkBuf[:dp.chunkBytes])
		dp.chunkBuf = dp.chunkBuf[dp.chunkBytes:]

		// remove, to do separately for discrete
		if dp.isStream {
			dp.buffer = append(dp.buffer, chunk...)
			if len(dp.buffer) > dp.maxBytes {
				dp.buffer = dp.buffer[dp.trimBytes:]
			}
		} else {
			dp.buffer = chunk
		}

		dp.newBytes += len(chunk)
	}
}

func (dp *DataProcessor) loop() {
	ticker := time.NewTicker(dp.sleepTime * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-dp.stop:
			return
		case <-ticker.C:
			if event := dp.tryEmit(); event != nil {
				dp.output <- event
			}
		}
	}
}

func (dp *DataProcessor) tryEmit() *Event {
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

func (dp *DataProcessor) Output() <-chan *Event {
	return dp.output
}
