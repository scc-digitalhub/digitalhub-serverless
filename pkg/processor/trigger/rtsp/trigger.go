/*
SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
SPDX-License-Identifier: Apache-2.0
*/

package rtsp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
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
)

// rtspTrigger streams media from a RTSP server, processes it in chunks,
// sends to Nuclio workers, and optionally forwards output to a webhook.
type rtspTrigger struct {
	trigger.AbstractTrigger
	configuration *Configuration

	client        *gortsplib.Client
	dataProcessor *DataProcessorStream
	pipeline      *MediaPipeline

	stop       chan struct{}
	wg         sync.WaitGroup
	webhookURL string
}

// NewTrigger creates a new RTSP trigger
func newTrigger(logger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration,
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
		stop:            make(chan struct{}),
	}
	t.Trigger = t

	return t, nil
}

// Start establishes RTSP connection, sets up the MediaPipeline, and starts processing
func (t *rtspTrigger) Start(checkpoint functionconfig.Checkpoint) error {
	t.Logger.InfoWith("Starting RTSP trigger", "url", t.configuration.RTSPURL)

	// configure webhook if specified
	if t.configuration.Output != nil {
		if kind, ok := t.configuration.Output["kind"].(string); ok && kind == "webhook" {
			if cfg, ok := t.configuration.Output["config"].(map[string]interface{}); ok {
				if url, ok := cfg["url"].(string); ok {
					t.webhookURL = url
					t.Logger.InfoWith("Webhook output configured", "url", t.webhookURL)
				}
			}
		}
	}

	// streaming processor
	t.dataProcessor = NewDataProcessorStream(
		t.configuration.ChunkBytes,
		t.configuration.MaxBytes,
		t.configuration.TrimBytes,
	)
	t.dataProcessor.Start(time.Millisecond * time.Duration(t.configuration.ProcessingInterval))

	// parse URL
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

	// setup media pipeline
	t.pipeline, err = NewMediaPipeline(desc.Medias)
	if err != nil {
		return errors.Wrap(err, "create media pipeline")
	}
	if err := t.pipeline.StartFFmpeg(16000, 1); err != nil {
		return errors.Wrap(err, "start FFmpeg")
	}

	// handle incoming RTP packets
	t.client.OnPacketRTPAny(func(media *description.Media, forma format.Format, pkt *rtp.Packet) {
		payload, err := t.pipeline.ProcessRTP(pkt, forma)
		if err != nil {
			t.Logger.WarnWith("RTP processing error", "err", err)
			return
		}
		if payload != nil && t.pipeline.ffmpegStdin != nil {
			_, _ = t.pipeline.ffmpegStdin.Write(payload)
		}
	})

	// read PCM output from FFmpeg and push to processor
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		buf := make([]byte, t.configuration.BufferSize)
		for {
			select {
			case <-t.stop:
				return
			default:
			}
			n, err := t.pipeline.ffmpegStdout.Read(buf)
			if err != nil {
				if err == io.EOF {
					t.Logger.Info("FFmpeg EOF")
				} else {
					t.Logger.WarnWith("FFmpeg read error", "err", err)
				}
				return
			}
			if n > 0 {
				t.dataProcessor.Push(buf[:n])
			}
		}
	}()

	// start dispatcher to workers & webhook
	t.wg.Add(1)
	go t.dispatcher()

	if _, err := t.client.Play(nil); err != nil {
		return errors.Wrap(err, "play RTSP stream")
	}

	return nil
}

// dispatcher waits for processed chunks and sends them to workers
func (t *rtspTrigger) dispatcher() {
	defer t.wg.Done()
	for {
		select {
		case <-t.stop:
			t.Logger.Info("RTSP dispatcher stopping")
			return
		case ev := <-t.dataProcessor.Output():
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

			if typedResp, ok := resp.(nuclio.Response); ok && t.webhookURL != "" {
				t.postToWebhook(typedResp.Body)
			}
		}
	}
}

// postToWebhook forwards processed event to the configured webhook
func (t *rtspTrigger) postToWebhook(body []byte) {
	payload := map[string]interface{}{
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

// Stop closes RTSP client, stops FFmpeg, processor, and dispatcher
func (t *rtspTrigger) Stop(force bool) (functionconfig.Checkpoint, error) {
	close(t.stop)

	if t.pipeline != nil {
		t.pipeline.StopFFmpeg()
	}

	if t.client != nil {
		t.client.Close()
	}

	if t.dataProcessor != nil {
		t.dataProcessor.Stop()
	}

	t.wg.Wait()
	t.Logger.Info("RTSP trigger stopped")
	return nil, nil
}

func (t *rtspTrigger) GetConfig() map[string]any {
	return common.StructureToMap(t.configuration)
}
