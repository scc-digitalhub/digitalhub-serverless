/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
*/

package rtsp

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

//
// ============================================================
// DATA PROCESSOR
// ============================================================
//

type DataProcessorStream struct {
	lock       sync.Mutex
	chunkBytes int
	maxBytes   int
	trimBytes  int
	chunkBuf   []byte
	buffer     []byte
	newBytes   int
	isVideo    bool
	output     chan *Event
	stop       chan struct{}
}

func NewDataProcessorStream(chunkBytes, maxBytes, trimBytes int, isVideo bool) *DataProcessorStream {
	return &DataProcessorStream{
		chunkBytes: chunkBytes,
		maxBytes:   maxBytes,
		trimBytes:  trimBytes,
		chunkBuf:   []byte{},
		buffer:     []byte{},
		isVideo:    isVideo,
		output:     make(chan *Event, 8),
		stop:       make(chan struct{}),
	}
}

func (dp *DataProcessorStream) Start(interval time.Duration) {
	go dp.loop(interval)
}

func (dp *DataProcessorStream) Stop() {
	close(dp.stop)
}

func (dp *DataProcessorStream) Push(data []byte) {
	dp.lock.Lock()
	defer dp.lock.Unlock()

	// video mode = replace frame
	if dp.isVideo {
		dp.buffer = make([]byte, len(data))
		copy(dp.buffer, data)
		dp.newBytes = len(data)
		return
	}

	// audio mode = chunk accumulate
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

func (dp *DataProcessorStream) loop(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-dp.stop:
			return
		case <-t.C:
			if ev := dp.tryEmit(); ev != nil {
				dp.output <- ev
			}
		}
	}
}

func (dp *DataProcessorStream) tryEmit() *Event {
	dp.lock.Lock()
	defer dp.lock.Unlock()

	if dp.isVideo {
		if dp.newBytes == 0 {
			return nil
		}
	} else {
		if dp.newBytes < dp.chunkBytes {
			return nil
		}
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

type MediaPipeline struct {
	trigger *rtspTrigger

	depacketizers map[uint8]any

	h264Decoders map[uint8]*OpenH264Decoder
	h264FirstIDR map[uint8]bool
}

func NewMediaPipeline(t *rtspTrigger, medias []*description.Media) (*MediaPipeline, error) {

	mp := &MediaPipeline{
		trigger:       t,
		depacketizers: make(map[uint8]any),
		h264Decoders:  make(map[uint8]*OpenH264Decoder),
		h264FirstIDR:  make(map[uint8]bool),
	}

	for _, media := range medias {
		for _, forma := range media.Formats {

			switch f := forma.(type) {

			// audio
			case *format.LPCM:
				dec, err := f.CreateDecoder()
				if err == nil {
					mp.depacketizers[forma.PayloadType()] = dec
				}

			// video
			case *format.H264:
				dep, err := f.CreateDecoder()
				if err != nil {
					continue
				}
				mp.depacketizers[forma.PayloadType()] = dep

				op, err := NewOpenH264Decoder()
				if err != nil {
					return nil, err
				}
				mp.h264Decoders[forma.PayloadType()] = op

				// feed SPS/PPS to decoder
				initNALUs := [][]byte{}
				if len(f.SPS) > 0 {
					initNALUs = append(initNALUs, f.SPS)
				}
				if len(f.PPS) > 0 {
					initNALUs = append(initNALUs, f.PPS)
				}
				if len(initNALUs) > 0 {
					op.Decode(initNALUs)
				}

				// t.dataProcessor.isVideo = true
				t.Logger.Info("Video stream detected (H264)")

			// ---------- H265 passthrough ----------
			case *format.H265:
				dep, err := f.CreateDecoder()
				if err == nil {
					mp.depacketizers[forma.PayloadType()] = dep
				}
				// t.dataProcessor.isVideo = true
			}
		}
	}

	return mp, nil
}

func (mp *MediaPipeline) ProcessRTP(pkt *rtp.Packet, forma format.Format) ([]byte, error) {

	dep, ok := mp.depacketizers[forma.PayloadType()]
	if !ok {
		return pkt.Payload, nil
	}

	if dec, ok := mp.h264Decoders[forma.PayloadType()]; ok {

		type h264Dep interface {
			Decode(*rtp.Packet) ([][]byte, error)
		}

		if hdec, ok := dep.(h264Dep); ok {

			au, err := hdec.Decode(pkt)
			if err != nil || au == nil {
				return nil, err
			}

			pt := forma.PayloadType()

			// WAIT FIRST IDR
			if !mp.h264FirstIDR[pt] {
				if !containsIDR(au) {
					return nil, nil
				}
				mp.h264FirstIDR[pt] = true
			}

			yuv, w, h, err := dec.Decode(au)
			if err != nil || yuv == nil {
				return nil, err
			}

			return EncodeFrameToJPEG(yuv, w, h, 80)
		}
	}

	// ===============================
	// GENERIC / AUDIO
	// ===============================
	switch d := dep.(type) {

	case interface {
		Decode(*rtp.Packet) ([]byte, error)
	}:
		payload, err := d.Decode(pkt)
		if err != nil || len(payload) == 0 {
			return nil, err
		}

		// convert big endian PCM to little endian (common format for audio processing)
		// go2rtp always streams in big endian
		payload = convertBigEndianToLittleEndian(payload)
		return payload, nil
	}

	return pkt.Payload, nil
}

func containsIDR(nalus [][]byte) bool {
	for _, n := range nalus {
		if len(n) == 0 {
			continue
		}
		if (n[0] & 0x1F) == 5 {
			return true
		}
	}
	return false
}

// JPEG encoding helper (for video frames)
func EncodeFrameToJPEG(yuv []byte, width, height int, quality int) ([]byte, error) {

	img := image.NewYCbCr(
		image.Rect(0, 0, width, height),
		image.YCbCrSubsampleRatio420,
	)

	ySize := width * height
	uvSize := ySize / 4

	copy(img.Y, yuv[:ySize])
	copy(img.Cb, yuv[ySize:ySize+uvSize])
	copy(img.Cr, yuv[ySize+uvSize:])

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ============================================================
// WEBHOOK
// ============================================================
func (t *rtspTrigger) postToWebhook(body []byte) {
	if t.webhookURL == "" {
		return
	}

	// Send webhook with multipart/form-data
	// body contains the handler response; typically text or binary data
	t.postToWebhookWithData("", body)
}

// postToWebhookWithData sends data to webhook using multipart/form-data
// text and data fields are optional (can be empty/nil)
func (t *rtspTrigger) postToWebhookWithData(text string, data []byte) {
	if t.webhookURL == "" {
		return
	}

	// Create multipart form
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)

	// Add text field if provided
	if text != "" {
		fw, err := writer.CreateFormField("text")
		if err != nil {
			t.Logger.WarnWith("Failed to create text form field", "err", err)
			return
		}
		if _, err := fw.Write([]byte(text)); err != nil {
			t.Logger.WarnWith("Failed to write text field", "err", err)
			return
		}
	}

	// Add data field if provided
	if len(data) > 0 {
		fw, err := writer.CreateFormFile("data", "frame.bin")
		if err != nil {
			t.Logger.WarnWith("Failed to create data form file", "err", err)
			return
		}
		if _, err := fw.Write(data); err != nil {
			t.Logger.WarnWith("Failed to write data file", "err", err)
			return
		}
	}

	if err := writer.Close(); err != nil {
		t.Logger.WarnWith("Failed to close multipart writer", "err", err)
		return
	}

	// Create and send request
	req, err := http.NewRequest("POST", t.webhookURL, buf)
	if err != nil {
		t.Logger.WarnWith("Failed to create webhook request", "err", err)
		return
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logger.WarnWith("Webhook POST failed", "err", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Logger.WarnWith("Webhook returned non-success status", "status", resp.StatusCode)
		return
	}

	t.Logger.DebugWith("Webhook POST succeeded", "status", resp.StatusCode, "text_len", len(text), "data_len", len(data))
}

func convertBigEndianToLittleEndian(in []byte) []byte {
	out := make([]byte, len(in))
	for i := 0; i+1 < len(in); i += 2 {
		v := binary.BigEndian.Uint16(in[i:])
		binary.LittleEndian.PutUint16(out[i:], v)
	}
	return out
}
