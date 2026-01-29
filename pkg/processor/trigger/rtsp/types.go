package rtsp

import (
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
)

const (
	DefaultBufferSize         = 4096
	DefaultChunkBytes         = 16000
	DefaultMaxBytes           = 1440000
	DefaultTrimBytes          = 1120000
	DefaultProcessingInterval = 2000
)

type Configuration struct {
	trigger.Configuration

	RTSPURL            string        `mapstructure:"rtsp_url"`
	BufferSize         int           `mapstructure:"buffer_size"`
	ChunkBytes         int           `mapstructure:"chunk_bytes"`
	MaxBytes           int           `mapstructure:"max_bytes"`
	TrimBytes          int           `mapstructure:"trim_bytes"`
	ProcessingInterval time.Duration `mapstructure:"processing_interval"`

	Output map[string]interface{} `mapstructure:"output"`
}

func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {

	// Defaults
	newConfiguration := Configuration{
		RTSPURL:            "",
		BufferSize:         DefaultBufferSize,
		ChunkBytes:         DefaultChunkBytes,
		MaxBytes:           DefaultMaxBytes,
		TrimBytes:          DefaultTrimBytes,
		ProcessingInterval: DefaultProcessingInterval,
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

	// if newConfiguration.BufferSize <= 0 {
	// 	newConfiguration.BufferSize = DefaultBufferSize
	// }

	// if newConfiguration.SampleRate <= 0 {
	// 	newConfiguration.SampleRate = DefaultSampleRate
	// }

	return &newConfiguration, nil
}
