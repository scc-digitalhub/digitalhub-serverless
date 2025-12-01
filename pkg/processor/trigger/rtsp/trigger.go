/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package rtsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"
)

type rtsp struct {
	trigger.AbstractTrigger
	configuration *Configuration
	events        []Event
	ffmpegCmd     *exec.Cmd
	ffmpegStdout  io.ReadCloser
	stopChan      chan struct{}
	wg            sync.WaitGroup
	processor     *AudioProcessor
	webhookURL    string
}

func newTrigger(logger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {

	abstractTrigger, err := trigger.NewAbstractTrigger(logger,
		workerAllocator,
		&configuration.Configuration,
		"async",
		"rtsp_webhook",
		configuration.Name,
		restartTriggerChan)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract trigger")
	}

	t := &rtsp{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
		stopChan:        make(chan struct{}),
	}
	t.Trigger = t
	t.allocateEvents(1)

	return t, nil
}

func (r *rtsp) Start(checkpoint functionconfig.Checkpoint) error {
	r.Logger.InfoWith("Starting RTSP listener trigger",
		"url", r.configuration.RTSPURL,
		"bufferSize", r.configuration.BufferSize,
		"sampleRate", r.configuration.SampleRate)

	fmt.Println(r.configuration.Output["kind"].(string))
	if r.configuration.Output != nil {
		kind, _ := r.configuration.Output["kind"].(string)
		config, _ := r.configuration.Output["config"].(map[string]interface{})
		if kind == "webhook" && config != nil {
			r.webhookURL, _ = config["url"].(string)
			r.Logger.InfoWith("Webhook output configured", "url", r.webhookURL)
		}
	}

	r.ffmpegCmd = exec.Command("ffmpeg",
		"-rtsp_transport", "tcp",
		"-i", r.configuration.RTSPURL,
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", r.configuration.SampleRate),
		"pipe:1",
	)
	r.ffmpegCmd.Stderr = os.Stderr

	var err error
	r.ffmpegStdout, err = r.ffmpegCmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "Failed to get FFmpeg stdout pipe")
	}

	if err := r.ffmpegCmd.Start(); err != nil {
		return errors.Wrap(err, "Failed to start FFmpeg process")
	}
	r.Logger.InfoWith("✓ FFmpeg started", "url", r.configuration.RTSPURL)

	r.processor = NewAudioProcessor(
		r.configuration.SampleRate,
		r.configuration.ChunkDurationSeconds,
		r.configuration.MaxBufferSeconds,
		r.configuration.TrimSeconds,
	)

	r.wg.Add(1)
	go r.readAudioPackets()

	return nil
}

func (r *rtsp) readAudioPackets() {
	defer r.wg.Done()

	buf := make([]byte, r.configuration.BufferSize)
	retryCount := 0
	maxRetries := 5

	for {
		select {
		case <-r.stopChan:
			r.Logger.InfoWith("✓ Audio packet reader stopped")
			return
		default:
		}

		n, err := r.ffmpegStdout.Read(buf)
		if err != nil && err != io.EOF {
			retryCount++
			if retryCount > maxRetries {
				r.Logger.ErrorWith("✗ Max retries exceeded, stopping reader", "error", err)
				return
			}
			r.Logger.WarnWith("⚠ Read error, retrying", "error", err, "retry", retryCount)
			time.Sleep(time.Second * time.Duration(retryCount))
			continue
		}
		retryCount = 0

		if n > 0 {
			chunks := r.processor.AddPCM(buf[:n])

			if len(chunks) > 0 {
				r.processor.lock.Lock()
				rollingBuffer := make([]byte, len(r.processor.buffer))
				copy(rollingBuffer, r.processor.buffer)
				r.processor.lock.Unlock()

				workerInstance, err := r.WorkerAllocator.Allocate(time.Second * 5)
				if err != nil {
					r.Logger.WarnWith("⚠ Failed to allocate worker", "error", err)
					continue
				}

				event := &Event{
					body:      rollingBuffer,
					timestamp: time.Now(),
					attributes: map[string]interface{}{
						"buffer-size": len(rollingBuffer),
						"chunks":      len(chunks),
					},
				}

				response, processErr := r.SubmitEventToWorker(r.Logger, workerInstance, event)
				r.WorkerAllocator.Release(workerInstance)

				if processErr != nil {
					r.Logger.WarnWith("⚠ Failed to process event", "error", processErr)
					continue
				}

				typedResponse, ok := response.(nuclio.Response)
				if !ok {
					r.Logger.Warn("⚠ Received non-nuclio response")
					continue
				}

				if typedResponse.StatusCode != 200 {
					r.Logger.WarnWith("⚠ Handler returned non-200 status", "statusCode", typedResponse.StatusCode)
					continue
				}

				if r.webhookURL != "" {
					r.postHandlerOutputToWebhook(typedResponse.Body)
				}

			}
		}

		if err == io.EOF {
			r.Logger.InfoWith("ℹ FFmpeg stream ended")
			return
		}
	}
}

func (r *rtsp) postHandlerOutputToWebhook(body []byte) {
	payload := map[string]interface{}{
		"handler_output": string(body), // wrap the string in a JSON object
	}
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", r.webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		r.Logger.WarnWith("⚠ Failed to create webhook POST request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.Logger.WarnWith("⚠ Failed to POST handler output to webhook", "error", err)
		return
	}
	defer resp.Body.Close()

	r.Logger.DebugWith("✓ Forwarded handler output to webhook", "statusCode", resp.StatusCode)
}

// Stop the trigger
func (r *rtsp) Stop(force bool) (functionconfig.Checkpoint, error) {
	r.Logger.DebugWith("Stopping RTSP listener trigger")

	close(r.stopChan)

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		r.Logger.DebugWith("✓ Readers stopped gracefully")
	case <-time.After(5 * time.Second):
		r.Logger.WarnWith("⚠ Reader stop timeout, forcing termination")
	}

	if r.ffmpegCmd != nil && r.ffmpegCmd.ProcessState == nil {
		if err := r.ffmpegCmd.Process.Kill(); err != nil {
			r.Logger.WarnWith("⚠ Failed to kill FFmpeg process", "error", err)
		}
		r.ffmpegCmd.Wait()
	}

	r.Logger.InfoWith("✓ RTSP trigger stopped")
	return nil, nil
}

func (r *rtsp) GetConfig() map[string]interface{} {
	return common.StructureToMap(r.configuration)
}

func (r *rtsp) allocateEvents(size int) {
	r.events = make([]Event, size)
	for i := 0; i < size; i++ {
		r.events[i] = Event{}
	}
}
