package extproc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
	t.Run("NewDefaultOptions", func(t *testing.T) {
		options := NewDefaultOptions()
		assert.NotNil(t, options)

		// Verify default values
		assert.Equal(t, "x-request-id", options.RequestIdHeaderName)
		assert.True(t, options.DecompressBodies)
		assert.True(t, options.BufferStreamedBodies)
		assert.False(t, options.UpdateExtProcHeader)
		assert.False(t, options.UpdateDurationHeader)

		// Verify unset values
		assert.False(t, options.LogStream)
		assert.False(t, options.LogPhases)
		assert.Equal(t, "", options.RequestIdFallback)
		assert.Equal(t, int64(1024*1024), options.PerRequestBodyBufferBytes)
	})

	t.Run("OptionsStructFields", func(t *testing.T) {
		options := &ProcessingOptions{
			LogStream:                 true,
			LogPhases:                 true,
			UpdateExtProcHeader:       true,
			UpdateDurationHeader:      true,
			RequestIdHeaderName:       "custom-request-id",
			RequestIdFallback:         "fallback-id",
			BufferStreamedBodies:      true,
			PerRequestBodyBufferBytes: -1,
			DecompressBodies:          false,
		}

		// Verify custom values
		assert.True(t, options.LogStream, "LogStream should be true")
		assert.True(t, options.LogPhases, "LogPhases should be true")
		assert.True(t, options.UpdateExtProcHeader, "UpdateExtProcHeader should be true")
		assert.True(t, options.UpdateDurationHeader, "UpdateDurationHeader should be true")
		assert.Equal(t, "custom-request-id", options.RequestIdHeaderName)
		assert.Equal(t, "fallback-id", options.RequestIdFallback)
		assert.True(t, options.BufferStreamedBodies)
		assert.Equal(t, int64(-1), options.PerRequestBodyBufferBytes)
		assert.False(t, options.DecompressBodies)
	})
}
