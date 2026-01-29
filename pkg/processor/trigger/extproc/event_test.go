package extproc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEvent(t *testing.T) {
	now := time.Now()

	// Create test request context with headers
	headers := AllHeaders{
		Headers: map[string]string{
			"Content-Type": "application/json",
			"X-Custom":     "custom-value",
		},
		RawHeaders: make(map[string][]byte),
	}

	ctx := &RequestContext{
		Method:     "POST",
		Path:       "/api/test",
		AllHeaders: &headers,
		Query: map[string][]string{
			"param1": {"value1"},
			"param2": {"1234"},
			"param3": {"value3a", "value3b"},
		},
		Started: now,
	}

	// Create test event
	event := &Event{
		ctx:  ctx,
		Body: []byte(`{"test":"body"}`),
	}

	t.Run("GetContentType", func(t *testing.T) {
		assert.Equal(t, "application/json", event.GetContentType())
	})

	t.Run("GetBody", func(t *testing.T) {
		assert.Equal(t, []byte(`{"test":"body"}`), event.GetBody())
	})

	t.Run("GetHeaderByteSlice", func(t *testing.T) {
		assert.Equal(t, []byte("custom-value"), event.GetHeaderByteSlice("X-Custom"))
	})

	t.Run("GetHeader", func(t *testing.T) {
		assert.Equal(t, []byte("custom-value"), event.GetHeader("X-Custom"))
	})

	t.Run("GetHeaders", func(t *testing.T) {
		headers := event.GetHeaders()
		assert.Equal(t, 2, len(headers))
		assert.Equal(t, []byte("custom-value"), headers["X-Custom"])
		assert.Equal(t, []byte("application/json"), headers["Content-Type"])
	})

	t.Run("GetHeaderString", func(t *testing.T) {
		assert.Equal(t, "custom-value", event.GetHeaderString("X-Custom"))
	})

	t.Run("GetMethod", func(t *testing.T) {
		assert.Equal(t, "POST", event.GetMethod())
	})

	t.Run("GetPath", func(t *testing.T) {
		assert.Equal(t, "/api/test", event.GetPath())
	})

	t.Run("GetFieldByteSlice", func(t *testing.T) {
		assert.Equal(t, []byte("value1"), event.GetFieldByteSlice("param1"))
		assert.Equal(t, []byte("value3a,value3b"), event.GetFieldByteSlice("param3"))
	})

	t.Run("GetFieldString", func(t *testing.T) {
		assert.Equal(t, "value1", event.GetFieldString("param1"))
		assert.Equal(t, "value3a,value3b", event.GetFieldString("param3"))
	})

	t.Run("GetFieldInt", func(t *testing.T) {
		value, err := event.GetFieldInt("param2")
		assert.NoError(t, err)
		assert.Equal(t, 1234, value)

		// Test non-existent field
		value, err = event.GetFieldInt("nonexistent")
		assert.NoError(t, err)
		assert.Equal(t, 0, value)

		// Test invalid integer
		ctx.Query["invalid"] = []string{"notanumber"}
		value, err = event.GetFieldInt("invalid")
		assert.Error(t, err)
	})

	t.Run("GetFields", func(t *testing.T) {
		fields := event.GetFields()
		assert.Equal(t, 4, len(fields))
		assert.Equal(t, "value1", fields["param1"])
		assert.Equal(t, "1234", fields["param2"])
		assert.Equal(t, "value3a,value3b", fields["param3"])
	})

	t.Run("GetTimestamp", func(t *testing.T) {
		assert.Equal(t, now, event.GetTimestamp())
	})
}
