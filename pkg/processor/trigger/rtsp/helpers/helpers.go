/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
*/

package helpers

import (
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

// MediaPipeline handles RTP depacketization and format-specific processing
type MediaPipeline struct {
	Depacketizers map[uint8]any
}

// NewMediaPipeline creates a media pipeline for processing RTP streams
func NewMediaPipeline(medias []*description.Media) (*MediaPipeline, error) {

	mp := &MediaPipeline{
		Depacketizers: make(map[uint8]any),
	}

	for _, media := range medias {
		for _, forma := range media.Formats {

			switch f := forma.(type) {

			// audio
			case *format.LPCM:
				dec, err := f.CreateDecoder()
				if err == nil {
					mp.Depacketizers[forma.PayloadType()] = dec
				}

			// video - depacketizer only; H264 decoder setup in NewVideoMediaPipeline
			case *format.H264:
				dep, err := f.CreateDecoder()
				if err != nil {
					continue
				}
				mp.Depacketizers[forma.PayloadType()] = dep

			// H265 passthrough
			case *format.H265:
				dep, err := f.CreateDecoder()
				if err == nil {
					mp.Depacketizers[forma.PayloadType()] = dep
				}
			}
		}
	}

	return mp, nil
}

// ProcessRTP handles generic RTP packet processing (audio, H265, etc.)
func (mp *MediaPipeline) ProcessRTP(pkt *rtp.Packet, forma format.Format) (any, error) {

	dep, ok := mp.Depacketizers[forma.PayloadType()]
	if !ok {
		return pkt.Payload, nil
	}

	// generic / audio / H265 passthrough
	switch d := dep.(type) {

	case interface {
		Decode(*rtp.Packet) ([]byte, error)
	}:
		payload, err := d.Decode(pkt)
		if err != nil || len(payload) == 0 {
			return nil, err
		}
		return payload, nil
	}

	return pkt.Payload, nil
}
