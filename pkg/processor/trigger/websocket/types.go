package websocket

import (
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
)

const (
	DefaultBufferSize         = 4096
	DefaultChunkBytes         = 160000
	DefaultMaxBytes           = 1440000
	DefaultTrimBytes          = 1120000
	DefaultProcessingInterval = 2000
	DefaultIsStream           = false
)

type Configuration struct {
	trigger.Configuration
	WebSocketAddr      string        `mapstructure:"websocket_addr"`
	BufferSize         int           `mapstructure:"buffer_size"`
	ChunkBytes         int           `mapstructure:"chunk_bytes"`
	MaxBytes           int           `mapstructure:"max_bytes"`
	TrimBytes          int           `mapstructure:"trim_bytes"`
	ProcessingInterval time.Duration `mapstructure:"processing_interval"`
	IsStream           bool          `mapstructure:"is_stream"`
}

func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {

	newConfiguration := Configuration{
		WebSocketAddr:      "",
		BufferSize:         DefaultBufferSize,
		ChunkBytes:         DefaultChunkBytes,
		MaxBytes:           DefaultMaxBytes,
		TrimBytes:          DefaultTrimBytes,
		ProcessingInterval: DefaultProcessingInterval,
		IsStream:           DefaultIsStream,
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
		return nil, errors.New("websocket_addr is required")
	}

	return &newConfiguration, nil
}
