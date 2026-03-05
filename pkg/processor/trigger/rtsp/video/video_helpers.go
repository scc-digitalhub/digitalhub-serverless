/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
*/

package video

import (
	"bytes"
	"image"
	"image/jpeg"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/nuclio/logger"
	"github.com/pion/rtp"
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/trigger/rtsp/helpers"
)

// VideoFrame represents a decoded video frame in YUV format with dimensions
type VideoFrame struct {
	YUV    []byte
	Width  int
	Height int
}

// VideoProcessor processes video data, holding the latest frame
// It extends BaseMediaProcessor with video-specific frame handling and JPEG encoding
type VideoProcessor struct {
	*helpers.BaseMediaProcessor
	videoFrame []byte
	videoW     int
	videoH     int
}

// VideoMediaPipeline extends MediaPipeline with H.264-specific decoder support
type VideoMediaPipeline struct {
	*helpers.MediaPipeline
	h264Decoders map[uint8]*OpenH264Decoder
	h264FirstIDR map[uint8]bool
}

// NewVideoProcessor creates a new video processor with specified buffer parameters
func NewVideoProcessor(chunkBytes, maxBytes, trimBytes int) *VideoProcessor {
	return &VideoProcessor{
		BaseMediaProcessor: helpers.NewBaseMediaProcessor(chunkBytes, maxBytes, trimBytes),
		videoFrame:         nil,
		videoW:             0,
		videoH:             0,
	}
}

// NewVideoMediaPipeline creates a video-specific pipeline with H.264 decoder support
func NewVideoMediaPipeline(rtspLogger logger.Logger, medias []*description.Media) (*VideoMediaPipeline, error) {

	mp, err := helpers.NewMediaPipeline(medias)
	if err != nil {
		return nil, err
	}

	vmp := &VideoMediaPipeline{
		MediaPipeline: mp,
		h264Decoders:  make(map[uint8]*OpenH264Decoder),
		h264FirstIDR:  make(map[uint8]bool),
	}

	// Initialize H.264 decoders for video formats
	for _, media := range medias {
		for _, forma := range media.Formats {
			if f, ok := forma.(*format.H264); ok {
				op, err := NewOpenH264Decoder()
				if err != nil {
					return nil, err
				}
				vmp.h264Decoders[forma.PayloadType()] = op

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

				rtspLogger.Info("Video stream detected (H264)")
			}
		}
	}

	return vmp, nil
}

// ProcessRTP handles H.264 video RTP packet processing for VideoProcessor
func (vp *VideoProcessor) ProcessRTP(pkt *rtp.Packet, forma format.Format) (interface{}, error) {
	pipeline := vp.BaseMediaProcessor.GetPipeline()
	if pipeline == nil {
		return pkt.Payload, nil
	}

	vmp, ok := pipeline.(*VideoMediaPipeline)
	if !ok {
		return pkt.Payload, nil
	}

	return vmp.ProcessRTP(pkt, forma)
}

// ProcessRTP_Internal handles H.264 video RTP packet processing with frame decoding.
// It is the internal implementation called by VideoProcessor.ProcessRTP.
func (vmp *VideoMediaPipeline) ProcessRTP(pkt *rtp.Packet, forma format.Format) (interface{}, error) {

	dep, ok := vmp.Depacketizers[forma.PayloadType()]
	if !ok {
		return pkt.Payload, nil
	}

	if dec, ok := vmp.h264Decoders[forma.PayloadType()]; ok {

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
			if !vmp.h264FirstIDR[pt] {
				if !containsIDR(au) {
					return nil, nil
				}
				vmp.h264FirstIDR[pt] = true
			}

			yuv, w, h, err := dec.Decode(au)
			if err != nil || yuv == nil {
				return nil, err
			}

			return VideoFrame{YUV: yuv, Width: w, Height: h}, nil
		}
	}

	// Fallback to generic processing for non-H264
	return vmp.MediaPipeline.ProcessRTP(pkt, forma)
}

// Push stores the latest video frame for processing
func (vp *VideoProcessor) Push(data interface{}) {
	vp.Lock.Lock()
	defer vp.Lock.Unlock()

	vf, ok := data.(VideoFrame)
	if !ok {
		// wrong type, ignore
		return
	}
	// keep a copy of the raw YUV frame and dimensions
	vp.videoFrame = make([]byte, len(vf.YUV))
	copy(vp.videoFrame, vf.YUV)
	vp.videoW = vf.Width
	vp.videoH = vf.Height
	vp.NewBytes = len(vf.YUV)
}

// tryEmit encodes the latest frame to JPEG and returns an event
func (vp *VideoProcessor) tryEmit() *helpers.Event {
	vp.Lock.Lock()
	defer vp.Lock.Unlock()

	if vp.videoFrame == nil {
		return nil
	}
	// convert the only frame we hold, then clear it
	jpeg, err := EncodeFrameToJPEG(vp.videoFrame, vp.videoW, vp.videoH, 80)
	if err != nil {
		return nil
	}
	vp.videoFrame = nil
	vp.NewBytes = 0
	return &helpers.Event{
		Body:      jpeg,
		Timestamp: time.Now(),
	}
}

// Start begins video processing with the specified interval
func (vp *VideoProcessor) Start(interval time.Duration) {
	vp.BaseStart(interval, vp.eventLoop)
}

// eventLoop is the main processing loop for video frames
func (vp *VideoProcessor) eventLoop(interval time.Duration) {
	vp.CommonEventLoop(interval, vp.tryEmit)
}

// EncodeFrameToJPEG converts a YUV frame to JPEG format with the specified quality
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

// containsIDR checks whether any of the supplied NALUs is an IDR frame
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
