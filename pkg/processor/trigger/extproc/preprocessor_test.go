package extproc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock EventHandler for testing
type MockEventHandler struct {
	mock.Mock
}

func (m *MockEventHandler) HandleEvent(ctx *RequestContext, body []byte) (*EventResponse, error) {
	args := m.Called(ctx, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EventResponse), args.Error(1)
}

func TestPreProcessor_GetName(t *testing.T) {
	processor := &PreProcessor{}
	assert.Equal(t, "preprocessor", processor.GetName())
}

func TestPreProcessor_ProcessRequestHeaders_NoBody(t *testing.T) {
	mockHandler := new(MockEventHandler)
	processor := &PreProcessor{
		AbstractProcessor: AbstractProcessor{
			Handler: mockHandler,
		},
	}

	headers := AllHeaders{
		Headers:    make(map[string]string),
		RawHeaders: make(map[string][]byte),
	}
	ctx := &RequestContext{
		AllHeaders: &headers,
	}

	// Test successful processing
	mockHandler.On("HandleEvent", ctx, []byte(nil)).Return(&EventResponse{
		Body: []byte("processed"),
	}, nil).Once()

	err := processor.ProcessRequestHeaders(ctx, headers)
	assert.NoError(t, err)
	mockHandler.AssertExpectations(t)
}

func TestPreProcessor_ProcessRequestBody(t *testing.T) {
	mockHandler := new(MockEventHandler)
	processor := &PreProcessor{
		AbstractProcessor: AbstractProcessor{
			Handler: mockHandler,
		},
	}

	headers := &AllHeaders{
		Headers:    make(map[string]string),
		RawHeaders: make(map[string][]byte),
	}
	ctx := &RequestContext{
		AllHeaders: headers,
		extProcOptions: &ProcessingOptions{
			RequestIdHeaderName: "x-request-id",
			RequestIdFallback:   "test-request-id",
		},
	}
	err := initReqCtx(ctx, headers)
	assert.NoError(t, err)

	body := []byte("test body")

	tests := []struct {
		name        string
		response    *EventResponse
		returnError error
	}{
		{
			name: "successful processing",
			response: &EventResponse{
				Body: []byte("processed body"),
			},
			returnError: nil,
		},
		{
			name:        "processing error",
			response:    nil,
			returnError: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler.On("HandleEvent", ctx, body).Return(tt.response, tt.returnError).Once()

			err := processor.ProcessRequestBody(ctx, body)
			assert.NoError(t, err) // Should continue request even on error
			mockHandler.AssertExpectations(t)
		})
	}
}

func TestPreProcessor_ProcessRequest(t *testing.T) {
	mockHandler := new(MockEventHandler)
	processor := &PreProcessor{
		AbstractProcessor: AbstractProcessor{
			Handler: mockHandler,
		},
	}

	ctx := &RequestContext{}
	body := []byte("test body")

	tests := []struct {
		name        string
		response    *EventResponse
		returnError error
		expected    []byte
	}{
		{
			name: "successful processing",
			response: &EventResponse{
				Body: []byte("processed body"),
			},
			returnError: nil,
			expected:    []byte("processed body"),
		},
		{
			name:        "processing error",
			response:    nil,
			returnError: assert.AnError,
			expected:    []byte("test body"), // Original body returned on error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler.On("HandleEvent", ctx, body).Return(tt.response, tt.returnError).Once()

			result, _, err := processor.processRequest(ctx, body)
			if tt.returnError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
			mockHandler.AssertExpectations(t)
		})
	}
}
