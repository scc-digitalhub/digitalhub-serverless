package websocket

import (
	"reflect"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
)

const (
	DefaultChunkBytes         = 160000
	DefaultMaxBytes           = 1440000
	DefaultTrimBytes          = 1120000
	DefaultProcessingInterval = 2000
	DefaultIsStream           = false
)

type Configuration struct {
	trigger.Configuration
	WebSocketAddr      string        `mapstructure:"websocket_addr"`
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
	if baseConfiguration == nil {
		return nil, errors.New("failed to create base trigger configuration")
	}
	newConfiguration.Configuration = *baseConfiguration

	// Use a strict decoder to surface unused/invalid attributes and handle
	// numeric -> time.Duration conversion
	decoderConfig := &mapstructure.DecoderConfig{
		ErrorUnused: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			func(f reflect.Type, t reflect.Type, data any) (any, error) {
				// Convert numeric types to time.Duration when target is time.Duration
				if t == reflect.TypeOf(time.Duration(0)) {
					switch v := data.(type) {
					case int:
						return time.Duration(v), nil
					case int64:
						return time.Duration(v), nil
					case float64:
						return time.Duration(int64(v)), nil
					}
				}
				return data, nil
			},
		),
		Result: &newConfiguration,
	}

	decoder, derr := mapstructure.NewDecoder(decoderConfig)
	if derr != nil {
		return nil, errors.Wrap(derr, "Failed to create mapstructure decoder for Websocket trigger attributes")
	}

	if err := decoder.Decode(triggerConfiguration.Attributes); err != nil {
		return nil, errors.Wrap(err, "Failed to decode Websocket trigger attributes")
	}

	if newConfiguration.WebSocketAddr == "" {
		return nil, errors.New("websocket_addr is required")
	}

	return &newConfiguration, nil
}
