/*
SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
*/

package rtsp

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

// MediaPipeline handles decoding RTP packets and optional FFmpeg conversion
type MediaPipeline struct {
	depacketizers map[uint8]interface{}

	ffmpegCmd    *exec.Cmd
	ffmpegStdin  io.WriteCloser
	ffmpegStdout io.ReadCloser

	mu sync.Mutex
}

// NewMediaPipeline creates a pipeline for given formats
// func NewMediaPipeline(medias []*format.Media) (*MediaPipeline, error) {
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
			case *format.MPEG1Audio:
				depacketizer, err = f.CreateDecoder()
				// Add more supported formats here
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
