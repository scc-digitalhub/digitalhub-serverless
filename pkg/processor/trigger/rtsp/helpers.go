/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
*/

package rtsp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

// DataProcessorStream buffers PCM data and emits fixed-size chunks
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
	chunkBytes, maxBytes, trimBytes int,
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

// MediaPipeline handles LPCM depacketizers
type MediaPipeline struct {
	depacketizers map[uint8]any
}

func NewMediaPipeline(t rtspTrigger, medias []*description.Media) (*MediaPipeline, error) {
	mp := &MediaPipeline{
		depacketizers: make(map[uint8]any),
	}

	for _, media := range medias {
		for _, forma := range media.Formats {
			switch f := forma.(type) {
			case *format.LPCM:
				t.Logger.Info("TEST 111", forma)
				dep, err := f.CreateDecoder()
				if err != nil {
					t.Logger.WarnWith("Failed to create LPCM decoder", "err", err)
					continue
				}
				mp.depacketizers[forma.PayloadType()] = dep
			}
		}
	}

	return mp, nil
}

// ProcessRTP decodes RTP packets using the depacketizer
func (mp *MediaPipeline) ProcessRTP(t rtspTrigger, pkt *rtp.Packet, forma format.Format) ([]byte, error) {
	dep, ok := mp.depacketizers[forma.PayloadType()]
	if !ok {
		// fallback: push payload directly
		return pkt.Payload, nil
	}

	switch d := dep.(type) {
	case interface {
		Decode(*rtp.Packet) ([]byte, error)
	}:
		// t.Logger.Info("TEST 222")
		payload, err := d.Decode(pkt)
		if err != nil {
			t.Logger.Info("TEST 555")
			return nil, err
		}
		if len(payload) == 0 {
			t.Logger.Info("TEST 666")
			return nil, nil
		}
		return payload, nil
	default:
		t.Logger.Info("TEST 000000000")

		return pkt.Payload, nil
	}
}

// postToWebhook forwards processed event to the configured webhook
func (t *rtspTrigger) postToWebhook(body []byte) {
	payload := map[string]any{
		"handler_output": string(body),
	}
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", t.webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		t.Logger.WarnWith("Failed to create webhook request", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logger.WarnWith("Failed to POST webhook request", "err", err)
		return
	}
	defer resp.Body.Close()
}
