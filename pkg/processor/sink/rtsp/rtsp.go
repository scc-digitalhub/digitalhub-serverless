// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
// SPDX-License-Identifier: Apache-2.0

package rtsp

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"sync"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/format/rtplpcm"
	"github.com/bluenviron/gortsplib/v5/pkg/format/rtpmjpeg"
	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/logger"
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink"
)

// Configuration for RTSP sink
type Configuration struct {
	Port       int    `json:"port,omitempty"`
	Path       string `json:"path,omitempty"`
	Type       string `json:"type,omitempty"` // "video" or "audio"
	SampleRate int    `json:"sample_rate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
}

// Sink implements RTSP streaming using gortsplib
type Sink struct {
	logger        logger.Logger
	configuration *Configuration
	server        *gortsplib.Server
	stream        *gortsplib.ServerStream
	mjpegFormat   *format.MJPEG
	mjpegEncoder  *rtpmjpeg.Encoder
	lpcmFormat    *format.LPCM
	lpcmEncoder   *rtplpcm.Encoder
	mutex         sync.RWMutex
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// factory implements sink.Factory
type factory struct{}

func (f *factory) Create(logger logger.Logger, configuration map[string]interface{}) (sink.Sink, error) {
	config := &Configuration{
		Port:       8554,
		Path:       "/stream",
		Type:       "video",
		SampleRate: 16000,
		Channels:   1,
	}

	if err := mapstructure.Decode(configuration, config); err != nil {
		return nil, fmt.Errorf("failed to parse rtsp sink configuration: %w", err)
	}

	if config.Type != "video" && config.Type != "audio" {
		return nil, fmt.Errorf("invalid rtsp type: %s (must be 'video' or 'audio')", config.Type)
	}

	return &Sink{
		logger:        logger,
		configuration: config,
		stopChan:      make(chan struct{}),
	}, nil
}

func (f *factory) GetKind() string {
	return "rtsp"
}

// Start starts the RTSP server
func (s *Sink) Start() error {
	s.logger.InfoWith("Starting RTSP sink",
		"port", s.configuration.Port,
		"path", s.configuration.Path,
		"type", s.configuration.Type)

	// Create media description based on type
	var desc description.Session

	if s.configuration.Type == "video" {
		// MJPEG video format
		s.mjpegFormat = &format.MJPEG{}
		desc = description.Session{
			Title: "DigitalHub MJPEG Stream",
			Medias: []*description.Media{{
				Type:    description.MediaTypeVideo,
				Formats: []format.Format{s.mjpegFormat},
			}},
		}

		// Create MJPEG encoder
		var err error
		s.mjpegEncoder, err = s.mjpegFormat.CreateEncoder()
		if err != nil {
			return fmt.Errorf("failed to create MJPEG encoder: %w", err)
		}
	} else {
		// LPCM audio format (little-endian PCM)
		s.lpcmFormat = &format.LPCM{
			PayloadTyp:   96,
			BitDepth:     16,
			SampleRate:   s.configuration.SampleRate,
			ChannelCount: s.configuration.Channels,
		}
		desc = description.Session{
			Title: "DigitalHub PCM Stream",
			Medias: []*description.Media{{
				Type:    description.MediaTypeAudio,
				Formats: []format.Format{s.lpcmFormat},
			}},
		}

		// Create LPCM encoder
		var err error
		s.lpcmEncoder, err = s.lpcmFormat.CreateEncoder()
		if err != nil {
			return fmt.Errorf("failed to create LPCM encoder: %w", err)
		}
	}

	// Create RTSP server
	s.server = &gortsplib.Server{
		Handler: &rtspHandler{
			sink:   s,
			logger: s.logger,
		},
		RTSPAddress:    fmt.Sprintf(":%d", s.configuration.Port),
		UDPRTPAddress:  ":8000", // RTP port for UDP transport
		UDPRTCPAddress: ":8001", // RTCP port for UDP transport
	}

	// Start the server first to initialize it
	if err := s.server.Start(); err != nil {
		return fmt.Errorf("failed to start RTSP server: %w", err)
	}

	// Now create the server stream after server is initialized
	s.stream = &gortsplib.ServerStream{
		Server: s.server,
		Desc:   &desc,
	}
	err := s.stream.Initialize()
	if err != nil {
		return fmt.Errorf("failed to start RTSP server: %w", err)
	}

	// Wait for server in a goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.server.Wait()
	}()

	return nil
}

// Stop stops the RTSP sink
func (s *Sink) Stop(force bool) error {
	s.logger.InfoWith("Stopping RTSP sink", "force", force)

	close(s.stopChan)

	if s.stream != nil {
		s.stream.Close()
	}

	if s.server != nil {
		s.server.Close()
	}

	s.wg.Wait()

	return nil
}

// Write sends data to the RTSP stream
func (s *Sink) Write(ctx context.Context, data []byte, metadata map[string]interface{}) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.stream == nil {
		return fmt.Errorf("rtsp sink not started")
	}

	if s.configuration.Type == "video" {
		return s.writeVideoFrame(data)
	}
	return s.writeAudioFrame(data)
}

// ensureJPEGDimensionsValid ensures JPEG dimensions are multiples of 8
// by decoding, cropping if needed, and re-encoding
func ensureJPEGDimensionsValid(jpegData []byte) ([]byte, error) {
	// Decode JPEG to get dimensions
	img, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode JPEG: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Check if dimensions are already valid
	if width%8 == 0 && height%8 == 0 {
		return jpegData, nil
	}

	// Crop to nearest multiple of 8 (round down)
	newWidth := (width / 8) * 8
	newHeight := (height / 8) * 8

	if newWidth == 0 || newHeight == 0 {
		return nil, fmt.Errorf("image too small to crop to valid dimensions (width=%d, height=%d)", width, height)
	}

	// Create cropped image
	croppedImg := img.(interface {
		SubImage(image.Rectangle) image.Image
	}).SubImage(image.Rect(0, 0, newWidth, newHeight))

	// Re-encode to JPEG
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, croppedImg, &jpeg.Options{Quality: 90})
	if err != nil {
		return nil, fmt.Errorf("failed to re-encode JPEG: %w", err)
	}

	return buf.Bytes(), nil
}

// writeVideoFrame writes a MJPEG frame to the stream
func (s *Sink) writeVideoFrame(jpegData []byte) error {
	if s.stream == nil || s.mjpegEncoder == nil {
		return fmt.Errorf("stream not initialized")
	}

	// Ensure JPEG dimensions are valid (multiples of 8)
	validJPEG, err := ensureJPEGDimensionsValid(jpegData)
	if err != nil {
		return fmt.Errorf("failed to validate JPEG dimensions: %w", err)
	}

	// Encode JPEG to RTP packets
	packets, err := s.mjpegEncoder.Encode(validJPEG)
	if err != nil {
		return fmt.Errorf("failed to encode MJPEG: %w", err)
	}

	// Write all RTP packets to the stream
	media := s.stream.Desc.Medias[0]
	for _, pkt := range packets {
		// err := s.stream.WritePacketRTPWithNTP(media, pkt, time.Now())
		err := s.stream.WritePacketRTP(media, pkt)
		if err != nil {
			return fmt.Errorf("failed to write RTP packet: %w", err)
		}
	}

	return nil
}

// writeAudioFrame writes a PCM audio frame to the stream
func (s *Sink) writeAudioFrame(pcmData []byte) error {
	if s.stream == nil || s.lpcmEncoder == nil {
		return fmt.Errorf("stream not initialized")
	}

	// Encode PCM to RTP packets
	packets, err := s.lpcmEncoder.Encode(pcmData)
	if err != nil {
		return fmt.Errorf("failed to encode LPCM: %w", err)
	}

	// Write all RTP packets to the stream
	media := s.stream.Desc.Medias[0]
	for _, pkt := range packets {
		s.logger.DebugWith("Writing audio RTP packet", "size", len(pkt.Payload))
		err := s.stream.WritePacketRTP(media, pkt)
		if err != nil {
			return fmt.Errorf("failed to write RTP packet: %w", err)
		}
	}

	return nil
}

// GetKind returns the sink type
func (s *Sink) GetKind() string {
	return "rtsp"
}

// GetConfig returns the sink configuration
func (s *Sink) GetConfig() map[string]interface{} {
	config := map[string]interface{}{
		"port": s.configuration.Port,
		"path": s.configuration.Path,
		"type": s.configuration.Type,
	}
	if s.configuration.Type == "audio" {
		config["sample_rate"] = s.configuration.SampleRate
		config["channels"] = s.configuration.Channels
	}
	return config
}

// rtspHandler implements gortsplib.ServerHandler
type rtspHandler struct {
	sink   *Sink
	logger logger.Logger
}

func (h *rtspHandler) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	h.logger.InfoWith("RTSP client connected", "remote", ctx.Conn.NetConn().RemoteAddr())
}

func (h *rtspHandler) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	h.logger.InfoWith("RTSP client disconnected", "error", ctx.Error)
}

func (h *rtspHandler) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	h.logger.InfoWith("RTSP session opened")
}

func (h *rtspHandler) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	h.logger.InfoWith("RTSP session closed")
}

func (h *rtspHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	h.logger.InfoWith("RTSP DESCRIBE request", "path", ctx.Path, "query", ctx.Query)

	// Return the stream if path matches
	if ctx.Path == h.sink.configuration.Path {
		return &base.Response{
			StatusCode: base.StatusOK,
		}, h.sink.stream, nil
	}

	return &base.Response{
		StatusCode: base.StatusNotFound,
	}, nil, nil
}

func (h *rtspHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	h.logger.InfoWith("RTSP SETUP request", "path", ctx.Path, "transport", ctx.Transport)

	// Validate path matches the configured stream path
	if ctx.Path != h.sink.configuration.Path {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, fmt.Errorf("path not found: %s", ctx.Path)
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, h.sink.stream, nil
}

func (h *rtspHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	h.logger.InfoWith("RTSP PLAY request")
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func (h *rtspHandler) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	return &base.Response{
		StatusCode: base.StatusNotImplemented,
	}, fmt.Errorf("recording not supported")
}

func (h *rtspHandler) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, *gortsplib.ServerStream, error) {
	return &base.Response{
		StatusCode: base.StatusNotImplemented,
	}, nil, fmt.Errorf("announce not supported")
}

func (h *rtspHandler) OnGetParameter(ctx *gortsplib.ServerHandlerOnGetParameterCtx) (*base.Response, error) {
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func (h *rtspHandler) OnSetParameter(ctx *gortsplib.ServerHandlerOnSetParameterCtx) (*base.Response, error) {
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func (h *rtspHandler) OnDecodeError(ctx *gortsplib.ServerHandlerOnDecodeErrorCtx) {
	h.logger.WarnWith("RTSP decode error", "error", ctx.Error)
}

func init() {
	sink.RegistrySingleton.Register("rtsp", &factory{})
}
