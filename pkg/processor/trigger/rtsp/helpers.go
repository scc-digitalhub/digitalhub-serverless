/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package rtsp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

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

// MediaPipeline handles decoding RTP packets and optional FFmpeg conversion
type MediaPipeline struct {
	depacketizers map[uint8]any

	ffmpegCmd    *exec.Cmd
	ffmpegStdin  io.WriteCloser
	ffmpegStdout io.ReadCloser

	mu sync.Mutex
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

// NewMediaPipeline creates a pipeline for given formats
func NewMediaPipeline(medias []*description.Media) (*MediaPipeline, error) {
	mp := &MediaPipeline{
		depacketizers: make(map[uint8]any),
	}

	// create depacketizers for supported formats
	for _, media := range medias {
		for _, forma := range media.Formats {
			var depacketizer any
			var err error

			switch f := forma.(type) {
			case *format.MPEG4Audio:
				depacketizer, err = f.CreateDecoder()
				// t.Logger.Info("TEST 000 MPEG 4")
			case *format.MPEG1Audio:
				depacketizer, err = f.CreateDecoder()
				// t.Logger.Info("TEST 111 MPEG 1")
				// add future formats here
			}

			if err == nil && depacketizer != nil {
				mp.depacketizers[forma.PayloadType()] = depacketizer
			}
		}
	}

	return mp, nil
}

// StartFFmpeg launches FFmpeg to convert input to PCM
func (mp *MediaPipeline) StartFFmpeg(sampleRate int, channels int) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	mp.ffmpegCmd = exec.Command("ffmpeg",
		"-loglevel", "warning",
		// "-f", "mp3",
		"-i", "pipe:0",
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-ac", strconv.Itoa(channels),
		"-ar", strconv.Itoa(sampleRate),
		"pipe:1",
	)
	var err error
	mp.ffmpegStdin, err = mp.ffmpegCmd.StdinPipe()
	if err != nil {
		return err
	}
	mp.ffmpegStdout, err = mp.ffmpegCmd.StdoutPipe()
	if err != nil {
		return err
	}
	mp.ffmpegCmd.Stderr = os.Stderr

	return mp.ffmpegCmd.Start()
}

// StopFFmpeg stops FFmpeg process
func (mp *MediaPipeline) StopFFmpeg() {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if mp.ffmpegStdin != nil {
		_ = mp.ffmpegStdin.Close()
	}
	if mp.ffmpegCmd != nil && mp.ffmpegCmd.ProcessState == nil {
		_ = mp.ffmpegCmd.Process.Kill()
		mp.ffmpegCmd.Wait()
	}
}

// ProcessRTP decodes an RTP packet and returns payload ready for FFmpeg
func (mp *MediaPipeline) ProcessRTP(pkt *rtp.Packet, forma format.Format) ([]byte, error) {
	depacketizer, ok := mp.depacketizers[forma.PayloadType()]
	if !ok {
		return pkt.Payload, nil
	}

	switch d := depacketizer.(type) {
	case interface {
		Decode(*rtp.Packet) ([][]byte, error)
	}:
		aus, err := d.Decode(pkt)
		if err != nil {
			return nil, err
		}
		if len(aus) == 0 {
			return nil, nil
		}
		return bytes.Join(aus, nil), nil
	default:
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
