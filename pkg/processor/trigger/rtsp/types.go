package rtsp

import (
	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
)

const (
	DefaultBufferSize           = 4096
	DefaultSampleRate           = 16000
	DefaultChunkDurationSeconds = 5
	DefaultMaxBufferSeconds     = 45
	DefaultTrimSeconds          = 30
)

type Configuration struct {
	trigger.Configuration

	RTSPURL              string                 `mapstructure:"rtsp_url"`
	BufferSize           int                    `mapstructure:"buffer_size"`
	SampleRate           int                    `mapstructure:"sample_rate"`
	ChunkDurationSeconds int                    `mapstructure:"chunk_duration_seconds"`
	MaxBufferSeconds     int                    `mapstructure:"max_buffer_seconds"`
	TrimSeconds          int                    `mapstructure:"trim_seconds"`
	Output               map[string]interface{} `mapstructure:"output"`
}

func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {

	// Defaults
	newConfiguration := Configuration{
		RTSPURL:              "",
		BufferSize:           DefaultMaxBufferSeconds,
		SampleRate:           DefaultSampleRate,
		ChunkDurationSeconds: DefaultChunkDurationSeconds,
		MaxBufferSeconds:     DefaultMaxBufferSeconds,
		TrimSeconds:          DefaultTrimSeconds,
	}

	// Base trigger config
	baseConfiguration, err := trigger.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create trigger configuration")
	}
	newConfiguration.Configuration = *baseConfiguration

	// Apply attributes
	if err := mapstructure.Decode(triggerConfiguration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode RTSP trigger attributes")
	}

	// Validate
	if newConfiguration.RTSPURL == "" {
		return nil, errors.New("rtspUrl is required")
	}

	if newConfiguration.BufferSize <= 0 {
		newConfiguration.BufferSize = DefaultBufferSize
	}

	if newConfiguration.SampleRate <= 0 {
		newConfiguration.SampleRate = DefaultSampleRate
	}

	return &newConfiguration, nil
}
