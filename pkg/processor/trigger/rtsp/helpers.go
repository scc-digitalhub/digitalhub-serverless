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
	isVideo    bool
	output     chan *Event
	stop       chan struct{}
}

type MediaPipeline struct {
	trigger *rtspTrigger

	depacketizers map[uint8]any

	h264Decoders map[uint8]*OpenH264Decoder
	h264FirstIDR map[uint8]bool
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

				t.Logger.Info("Video stream detected (H264)")

			// H265 passthrough
			case *format.H265:
				dep, err := f.CreateDecoder()
				if err == nil {
					mp.depacketizers[forma.PayloadType()] = dep
				}
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

			// wait first IDR
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

	// generic / audio
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

func convertBigEndianToLittleEndian(in []byte) []byte {
	out := make([]byte, len(in))
	for i := 0; i+1 < len(in); i += 2 {
		v := binary.BigEndian.Uint16(in[i:])
		binary.LittleEndian.PutUint16(out[i:], v)
	}
	return out
}
