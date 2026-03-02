/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
*/

package rtsp

import (
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

type VideoFrame struct {
	YUV    []byte
	Width  int
	Height int
}

// MediaProcessor is the interface for processing media data (audio or video)
type MediaProcessor interface {
	Start(interval time.Duration)
	Stop()
	Push(data any)
	Output() <-chan *Event
}

// DataProcessor is the base for both audio and video processors.
// It handles the event loop and output channel management.
type DataProcessor struct {
	lock       sync.Mutex
	chunkBytes int
	maxBytes   int
	trimBytes  int
	chunkBuf   []byte
	newBytes   int
	output     chan *Event
	stop       chan struct{}
}

type MediaPipeline struct {
	trigger *rtspTrigger

	depacketizers map[uint8]any

	h264Decoders map[uint8]*OpenH264Decoder
	h264FirstIDR map[uint8]bool
}

// Common methods for all processors

func (dp *DataProcessor) Start(interval time.Duration) {
	go dp.loop(interval)
}

func (dp *DataProcessor) Stop() {
	close(dp.stop)
}

func (dp *DataProcessor) Output() <-chan *Event {
	return dp.output
}

func (dp *DataProcessor) loop(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-dp.stop:
			return
		case <-t.C:
			// This will be called by the subclass-specific tryEmit logic
			// via Select statement in Start
		}
	}
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

func (mp *MediaPipeline) ProcessRTP(pkt *rtp.Packet, forma format.Format) (interface{}, error) {

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

			return VideoFrame{YUV: yuv, Width: w, Height: h}, nil
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
