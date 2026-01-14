/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package websocket

import (
	"sync"
	"time"
)

// DataProcessorDiscrete processes independent WebSocket messages.
// Each Push replaces the previous value, and latest data
// is periodically emitted as an Event.
type DataProcessorDiscrete struct {
	lock               sync.Mutex
	data               []byte
	hasNewData         bool
	processingInterval time.Duration
	output             chan *Event
	stop               chan struct{}
}

func NewDataProcessorDiscrete(processingInterval time.Duration) *DataProcessorDiscrete {
	return &DataProcessorDiscrete{
		output:             make(chan *Event, 8),
		stop:               make(chan struct{}),
		processingInterval: processingInterval,
		hasNewData:         false,
		data:               nil,
	}
}

func (dp *DataProcessorDiscrete) Start() {
	go dp.loop()
}

func (dp *DataProcessorDiscrete) Stop() {
	close(dp.stop)
}

func (dp *DataProcessorDiscrete) Push(data []byte) {
	dp.lock.Lock()
	defer dp.lock.Unlock()

	buf := make([]byte, len(data))
	copy(buf, data)

	dp.data = buf
	dp.hasNewData = true
}

func (dp *DataProcessorDiscrete) loop() {
	ticker := time.NewTicker(dp.processingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-dp.stop:
			return
		case <-ticker.C:
			ev := dp.tryEmit()
			if ev != nil {
				dp.output <- ev
			}
		}
	}
}

// emit only if new data was received since last emission
func (dp *DataProcessorDiscrete) tryEmit() *Event {
	dp.lock.Lock()
	defer dp.lock.Unlock()

	if !dp.hasNewData || len(dp.data) == 0 {
		return nil
	}

	snapshot := make([]byte, len(dp.data))
	copy(snapshot, dp.data)

	dp.hasNewData = false

	return &Event{
		body:      snapshot,
		timestamp: time.Now(),
	}
}

func (dp *DataProcessorDiscrete) Output() <-chan *Event {
	return dp.output
}
