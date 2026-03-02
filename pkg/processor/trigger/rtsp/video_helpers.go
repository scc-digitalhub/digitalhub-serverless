/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
*/

package rtsp

import (
	"bytes"
	"image"
	"image/jpeg"
	"time"
)

// VideoProcessor processes video data, holding the latest frame
// and converting to JPEG on emission
type VideoProcessor struct {
	*DataProcessor
	videoFrame []byte
	videoW     int
	videoH     int
}

func NewVideoProcessor(chunkBytes, maxBytes, trimBytes int) *VideoProcessor {
	return &VideoProcessor{
		&DataProcessor{
			chunkBytes: chunkBytes,
			maxBytes:   maxBytes,
			trimBytes:  trimBytes,
			chunkBuf:   []byte{},
			output:     make(chan *Event, 8),
			stop:       make(chan struct{}),
		},
		nil, 0, 0,
	}
}

func (vp *VideoProcessor) Push(data any) {
	vp.lock.Lock()
	defer vp.lock.Unlock()

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
	vp.newBytes = len(vf.YUV)
}

func (vp *VideoProcessor) tryEmit() *Event {
	vp.lock.Lock()
	defer vp.lock.Unlock()

	if vp.videoFrame == nil {
		return nil
	}
	// convert the only frame we hold, then clear it
	jpeg, err := EncodeFrameToJPEG(vp.videoFrame, vp.videoW, vp.videoH, 80)
	if err != nil {
		return nil
	}
	vp.videoFrame = nil
	vp.newBytes = 0
	return &Event{
		body:      jpeg,
		timestamp: time.Now(),
	}
}

func (vp *VideoProcessor) Start(interval time.Duration) {
	go vp.videoLoop(interval)
}

func (vp *VideoProcessor) videoLoop(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-vp.stop:
			return
		case <-t.C:
			if ev := vp.tryEmit(); ev != nil {
				vp.output <- ev
			}
		}
	}
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
