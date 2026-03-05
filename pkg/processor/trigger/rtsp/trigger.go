/*
SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
*/

package rtsp

import (
	"context"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/pion/rtp"
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/sink"
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/trigger/rtsp/audio"
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/trigger/rtsp/helpers"
	"github.com/scc-digitalhub/digitalhub-serverless/pkg/processor/trigger/rtsp/video"
)

// rtspTrigger is the main RTSP streaming trigger.
// It owns configuration, runtime state, and media processing.
type rtspTrigger struct {
	trigger.AbstractTrigger
	configuration  *helpers.Configuration
	sink           sink.Sink
	client         *gortsplib.Client
	mediaProcessor helpers.MediaProcessor
	wg             sync.WaitGroup
}

// NewTrigger creates a new RTSP trigger instance
func newTrigger(logger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *helpers.Configuration,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {

	abstract, err := trigger.NewAbstractTrigger(
		logger,
		workerAllocator,
		&configuration.Configuration,
		"async",
		"rtsp",
		configuration.Name,
		restartTriggerChan,
	)
	if err != nil {
		return nil, errors.Wrap(err, "abstract trigger")
	}

	t := &rtspTrigger{
		AbstractTrigger: abstract,
		configuration:   configuration,
		client:          nil,
		mediaProcessor:  nil,
	}
	// set the trigger interface to our wrapper so methods dispatch correctly
	t.Trigger = t

	if configuration.Sink != nil && configuration.Sink.Kind != "" {
		sinkInstance, err := sink.RegistrySingleton.Create(
			logger,
			configuration.Sink.Kind,
			configuration.Sink.Attributes,
		)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create sink")
		}
		t.sink = sinkInstance
		logger.InfoWith("Sink configured for RTSP trigger", "kind", configuration.Sink.Kind)
	}

	return t, nil
}

func (t *rtspTrigger) Start(checkpoint functionconfig.Checkpoint) error {
	// logging and config up to now
	t.Logger.InfoWith("Starting RTSP trigger", "url", t.configuration.RTSPURL)

	if t.sink != nil {
		if err := t.sink.Start(); err != nil {
			return errors.Wrap(err, "Failed to start sink")
		}
		t.Logger.InfoWith("Sink started", "kind", t.sink.GetKind())
	}

	// Create appropriate media processor based on media type
	t.mediaProcessor = t.createMediaProcessor()
	t.mediaProcessor.Start(time.Millisecond * time.Duration(t.configuration.ProcessingInterval))

	u, err := base.ParseURL(t.configuration.RTSPURL)
	if err != nil {
		return errors.Wrap(err, "parse RTSP URL")
	}

	// gortsplib client
	t.client = &gortsplib.Client{
		Scheme: "rtsp",
		Host:   u.Host,
	}

	if err := t.client.Start(); err != nil {
		return errors.Wrap(err, "start RTSP client")
	}

	desc, _, err := t.client.Describe(u)
	if err != nil {
		return errors.Wrap(err, "describe RTSP URL")
	}

	if err := t.client.SetupAll(desc.BaseURL, desc.Medias); err != nil {
		return errors.Wrap(err, "setup all medias")
	}

	// Initialize media-specific pipeline and RTP processor
	if err := t.setupMediaPipeline(desc.Medias); err != nil {
		return err
	}

	// handle incoming RTP packets using polymorphic processor
	t.client.OnPacketRTPAny(func(media *description.Media, forma format.Format, pkt *rtp.Packet) {
		payload, err := t.mediaProcessor.ProcessRTP(pkt, forma)
		if err != nil {
			return
		}
		if payload != nil {
			t.mediaProcessor.Push(payload)
		}
	})

	// start dispatcher to workers & webhook
	t.wg.Add(1)
	go t.dispatcher()

	if _, err := t.client.Play(nil); err != nil {
		return errors.Wrap(err, "play RTSP stream")
	}

	return nil
}

// createMediaProcessor creates the appropriate media processor based on configuration
func (t *rtspTrigger) createMediaProcessor() helpers.MediaProcessor {
	switch t.configuration.MediaType {
	case "audio":
		return audio.NewAudioProcessor(
			t.configuration.ChunkBytes,
			t.configuration.MaxBytes,
			t.configuration.TrimBytes,
		)
	case "video":
		return video.NewVideoProcessor(
			t.configuration.ChunkBytes,
			t.configuration.MaxBytes,
			t.configuration.TrimBytes,
		)
	default:
		t.Logger.WarnWith("Unknown media type, defaulting to audio", "mediaType", t.configuration.MediaType)
		return audio.NewAudioProcessor(
			t.configuration.ChunkBytes,
			t.configuration.MaxBytes,
			t.configuration.TrimBytes,
		)
	}
}

// setupMediaPipeline initializes the media-specific pipeline
// Uses polymorphism to handle both audio and video formats
func (t *rtspTrigger) setupMediaPipeline(medias []*description.Media) error {
	switch t.configuration.MediaType {
	case "video":
		vmp, err := video.NewVideoMediaPipeline(t.Logger, medias)
		if err != nil {
			return errors.Wrap(err, "create video media pipeline")
		}
		t.mediaProcessor.SetPipeline(vmp)
	case "audio":
		mp, err := helpers.NewMediaPipeline(medias)
		if err != nil {
			return errors.Wrap(err, "create media pipeline")
		}
		t.mediaProcessor.SetPipeline(mp)
	default:
		return errors.New("unsupported media type: " + t.configuration.MediaType)
	}
	return nil
}

// dispatcher waits for processed media chunks and sends them to workers
// Uses polymorphism - the processor handles all audio/video specific logic
func (t *rtspTrigger) dispatcher() {
	defer t.wg.Done()
	for ev := range t.mediaProcessor.Output() {
		if ev == nil {
			continue
		}

		workerInstance, err := t.WorkerAllocator.Allocate(5 * time.Second)
		if err != nil {
			t.Logger.WarnWith("Worker allocation failed", "err", err)
			continue
		}

		resp, err := t.SubmitEventToWorker(t.Logger, workerInstance, ev)
		t.WorkerAllocator.Release(workerInstance)
		if err != nil {
			t.Logger.WarnWith("SubmitEventToWorker failed", "err", err)
			continue
		}

		var responseData []byte
		if typedResp, ok := resp.(nuclio.Response); ok {
			responseData = typedResp.Body
		}

		if t.sink != nil && len(responseData) > 0 {
			metadata := map[string]interface{}{
				"timestamp": ev.Timestamp,
			}
			if err := t.sink.Write(context.Background(), responseData, metadata); err != nil {
				t.Logger.WarnWith("Failed to write to sink", "error", err)
			}
		}
	}
	t.Logger.Info("RTSP dispatcher stopping")
}

// Stop closes RTSP client, stops processor and dispatcher
func (t *rtspTrigger) Stop(force bool) (functionconfig.Checkpoint, error) {
	if t.client != nil {
		t.client.Close()
	}

	if t.mediaProcessor != nil {
		t.mediaProcessor.Stop()
	}

	t.wg.Wait()

	if t.sink != nil {
		if err := t.sink.Stop(force); err != nil {
			t.Logger.WarnWith("Failed to stop sink", "error", err)
		}
	}

	t.Logger.Info("RTSP trigger stopped")
	return nil, nil
}

func (t *rtspTrigger) GetConfig() map[string]interface{} {
	return common.StructureToMap(t.configuration)
}
