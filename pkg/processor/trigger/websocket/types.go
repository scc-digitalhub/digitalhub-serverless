package websocket

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
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
	WebSocketAddr        string `mapstructure:"websocket_addr"`
	BufferSize           int    `mapstructure:"buffer_size"`
	SampleRate           int    `mapstructure:"sample_rate"`
	ChunkDurationSeconds int    `mapstructure:"chunk_duration_seconds"`
	MaxBufferSeconds     int    `mapstructure:"max_buffer_seconds"`
	TrimSeconds          int    `mapstructure:"trim_seconds"`
}

func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {

	newConfiguration := Configuration{
		WebSocketAddr:        "",
		BufferSize:           DefaultBufferSize,
		SampleRate:           DefaultSampleRate,
		ChunkDurationSeconds: DefaultChunkDurationSeconds,
		MaxBufferSeconds:     DefaultMaxBufferSeconds,
		TrimSeconds:          DefaultTrimSeconds,
	}

	baseConfiguration, err := trigger.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create trigger configuration")
	}
	newConfiguration.Configuration = *baseConfiguration

	if err := mapstructure.Decode(triggerConfiguration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode Websocket trigger attributes")
	}

	if newConfiguration.WebSocketAddr == "" {
		return nil, errors.New("websocketAddr is required")
	}

	return &newConfiguration, nil
}
