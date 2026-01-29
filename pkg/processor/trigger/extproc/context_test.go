package extproc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext(t *testing.T) {
	headers := &AllHeaders{
		Headers: map[string]string{
			":scheme":        "https",
			":authority":     "example.com",
			":method":        "POST",
			":path":          "/api/data?key1=value1&key2=value2",
			"x-request-id":   "test-request-id",
			"content-type":   "application/json",
			"content-length": "10",
		},
		RawHeaders: map[string][]byte{
			"x-binary": {0xFF, 0xFE, 0xFD},
		},
	}

	t.Run("context initialization", func(t *testing.T) {
		ctx := &RequestContext{
			AllHeaders: headers,
			extProcOptions: &ProcessingOptions{
				RequestIdHeaderName: "x-request-id",
				RequestIdFallback:   "fallback-id",
			},
		}
		err := initReqCtx(ctx, headers)
		require.NoError(t, err)

		assert.Equal(t, "https", ctx.Scheme)
		assert.Equal(t, "example.com", ctx.Authority)
		assert.Equal(t, "POST", ctx.Method)
		assert.Equal(t, "/api/data", ctx.Path)
		assert.Equal(t, "test-request-id", ctx.RequestID)
		assert.NotNil(t, ctx.Query)
		assert.Equal(t, "value1", ctx.Query["key1"][0])
		assert.Equal(t, "value2", ctx.Query["key2"][0])
	})

	t.Run("body handling", func(t *testing.T) {
		ctx := &RequestContext{
			AllHeaders: headers,
			bodybuffer: &EncodedBody{
				Value: make([]byte, 0),
				Type: BodyType{
					ContentType: "application/json",
				},
			},
		}

		assert.True(t, ctx.HasBody())

		err := ctx.appendBodyChunk([]byte("test data"))
		require.NoError(t, err)
		assert.Equal(t, "test data", string(ctx.CurrentBodyBytes()))
	})

	t.Run("header manipulation", func(t *testing.T) {
		ctx := &RequestContext{
			AllHeaders: headers,
		}
		ctx.ResetPhase()

		// Test header addition
		err := ctx.AddHeader("x-test", HeaderValue{Value: "test-value"})
		require.NoError(t, err)

		// Test header overwrite
		err = ctx.OverwriteHeader("x-test", HeaderValue{Value: "new-value"})
		require.NoError(t, err)

		// Test header removal
		err = ctx.RemoveHeader("x-test")
		require.NoError(t, err)
	})

	t.Run("value storage", func(t *testing.T) {
		ctx := &RequestContext{
			AllHeaders: headers,
			data:       make(map[string]interface{}), // Initialize the data map
		}
		ctx.ResetPhase()

		err := ctx.SetStoredValue("testKey", "testValue")
		require.NoError(t, err)

		assert.True(t, ctx.HasStoredValue("testKey"))
		value, err := ctx.GetStoredValue("testKey")
		require.NoError(t, err)
		assert.Equal(t, "testValue", value)

		// Test non-existent key
		_, err = ctx.GetStoredValue("nonexistent")
		assert.Error(t, err)
	})

	t.Run("request continuation and cancellation", func(t *testing.T) {
		ctx := &RequestContext{
			AllHeaders: headers,
		}
		ctx.ResetPhase()

		// Test continue request
		err := ctx.ContinueRequest()
		require.NoError(t, err)
		assert.NotNil(t, ctx.response.continueRequest)

		// Test cancel request
		err = ctx.CancelRequest(403, map[string]HeaderValue{
			"x-error": {Value: "test-error"},
		}, []byte("error message"))
		require.NoError(t, err)
		assert.NotNil(t, ctx.response.immediateResponse)
		assert.Equal(t, int32(403), int32(ctx.response.immediateResponse.Status.Code))
	})

	t.Run("response phase handling", func(t *testing.T) {
		ctx := &RequestContext{
			AllHeaders: headers,
		}
		ctx.ResetPhase()

		// Test request headers phase
		resp, err := ctx.GetResponse(REQUEST_PHASE_REQUEST_HEADERS)
		require.NoError(t, err)
		assert.NotNil(t, resp.GetRequestHeaders())

		// Test request body phase
		resp, err = ctx.GetResponse(REQUEST_PHASE_REQUEST_BODY)
		require.NoError(t, err)
		assert.NotNil(t, resp.GetRequestBody())

		// Test invalid phase
		_, err = ctx.GetResponse(999)
		assert.Error(t, err)
	})
}
