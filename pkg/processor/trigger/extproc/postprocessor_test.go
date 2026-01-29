package extproc

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHandler struct {
	response []byte
	err      error
}

func (h *mockHandler) HandleEvent(ctx *RequestContext, body []byte) (*EventResponse, error) {
	if h.err != nil {
		return nil, h.err
	}
	return &EventResponse{
		Status:  0,
		Headers: nil,
		Body:    h.response,
	}, nil
}

func TestPostprocessor(t *testing.T) {
	tests := []struct {
		name          string
		handler       *mockHandler
		inputBody     []byte
		expectedBody  []byte
		expectedError error
		shouldReplace bool
	}{
		{
			name: "successful processing",
			handler: &mockHandler{
				response: []byte("processed"),
				err:      nil,
			},
			inputBody:     []byte("original"),
			expectedBody:  []byte("processed"),
			expectedError: nil,
			shouldReplace: true,
		},
		{
			name: "handler error",
			handler: &mockHandler{
				response: nil,
				err:      errors.New("handler error"),
			},
			inputBody:     nil,
			expectedBody:  nil,
			expectedError: nil,
			shouldReplace: false,
		},
		{
			name: "empty response",
			handler: &mockHandler{
				response: []byte{},
				err:      nil,
			},
			inputBody:     []byte("original"),
			expectedBody:  nil,
			expectedError: nil,
			shouldReplace: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create postprocessor with mock handler
			processor := &PostProcessor{}
			processor.Handler = tt.handler

			// Create request context
			ctx := &RequestContext{
				AllHeaders: &AllHeaders{},
			}

			// Test name
			assert.Equal(t, "postprocessor", processor.GetName())

			// Test header processing
			err := processor.ProcessResponseHeaders(ctx, *ctx.AllHeaders)
			require.NoError(t, err)

			// Test body processing
			err = processor.ProcessResponseBody(ctx, tt.inputBody)
			require.NoError(t, err)

			if tt.shouldReplace {
				assert.Equal(t, tt.expectedBody, ctx.response.bodyMutation.GetBody())
			} else {
				assert.Equal(t, tt.inputBody, ctx.response.bodyMutation.GetBody())
			}
		})
	}
}
