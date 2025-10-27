package extproc

import (
	"context"
	"net"
	"testing"
	"time"

	epb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

// Mock RequestProcessor implementation
type MockRequestProcessor struct {
	mock.Mock
}

func (m *MockRequestProcessor) GetName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockRequestProcessor) GetOptions() *ProcessingOptions {
	args := m.Called()
	return args.Get(0).(*ProcessingOptions)
}

func (m *MockRequestProcessor) ProcessRequestHeaders(ctx *RequestContext, headers AllHeaders) error {
	args := m.Called(ctx, headers)
	return args.Error(0)
}

func (m *MockRequestProcessor) ProcessRequestTrailers(ctx *RequestContext, trailers AllHeaders) error {
	args := m.Called(ctx, trailers)
	return args.Error(0)
}

func (m *MockRequestProcessor) ProcessResponseHeaders(ctx *RequestContext, headers AllHeaders) error {
	args := m.Called(ctx, headers)
	return args.Error(0)
}

func (m *MockRequestProcessor) ProcessResponseTrailers(ctx *RequestContext, trailers AllHeaders) error {
	args := m.Called(ctx, trailers)
	return args.Error(0)
}

func (m *MockRequestProcessor) ProcessRequestBody(ctx *RequestContext, body []byte) error {
	args := m.Called(ctx, body)
	return args.Error(0)
}

func (m *MockRequestProcessor) ProcessResponseBody(ctx *RequestContext, body []byte) error {
	args := m.Called(ctx, body)
	return args.Error(0)
}

func TestDefaultServerOptions(t *testing.T) {
	opts := DefaultServerOptions()
	assert.Equal(t, 15, opts.GracefulShutdownTimeout)
	assert.Equal(t, uint32(100), opts.MaxConcurrentStreams)
}

func TestGenericExtProcServer(t *testing.T) {
	mockProcessor := new(MockRequestProcessor)
	mockProcessor.On("GetName").Return("test-processor")
	defaultOpts := NewDefaultOptions()
	mockProcessor.On("GetOptions").Return(defaultOpts)
	mockProcessor.On("ProcessRequestHeaders", mock.Anything, mock.Anything).Return(nil)
	mockProcessor.On("ProcessResponseHeaders", mock.Anything, mock.Anything).Return(nil)
	mockProcessor.On("ProcessRequestBody", mock.Anything, mock.Anything).Return(nil)
	mockProcessor.On("ProcessResponseBody", mock.Anything, mock.Anything).Return(nil)
	mockProcessor.On("ProcessRequestTrailers", mock.Anything, mock.Anything).Return(nil)
	mockProcessor.On("ProcessResponseTrailers", mock.Anything, mock.Anything).Return(nil)

	name := mockProcessor.GetName()
	opts := mockProcessor.GetOptions()
	server := &GenericExtProcServer{
		name:      name,
		processor: mockProcessor,
		options:   opts,
	}

	// Create a buffer for testing gRPC
	listener := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	epb.RegisterExternalProcessorServer(s, server)

	go func() {
		if err := s.Serve(listener); err != nil {
			t.Error(err)
		}
	}()

	// Create a client connection
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithContextDialer(
		func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithInsecure(),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := epb.NewExternalProcessorClient(conn)
	stream, err := client.Process(ctx)
	require.NoError(t, err)

	// Test stream processing by sending various types of requests
	// Send request headers
	err = stream.Send(&epb.ProcessingRequest{
		Request: &epb.ProcessingRequest_RequestHeaders{},
	})
	require.NoError(t, err)

	// Send request body
	err = stream.Send(&epb.ProcessingRequest{
		Request: &epb.ProcessingRequest_RequestBody{},
	})
	require.NoError(t, err)

	// Send request trailers
	err = stream.Send(&epb.ProcessingRequest{
		Request: &epb.ProcessingRequest_RequestTrailers{},
	})
	require.NoError(t, err)

	// Send response headers
	err = stream.Send(&epb.ProcessingRequest{
		Request: &epb.ProcessingRequest_ResponseHeaders{},
	})
	require.NoError(t, err)

	// Send response body
	err = stream.Send(&epb.ProcessingRequest{
		Request: &epb.ProcessingRequest_ResponseBody{},
	})
	require.NoError(t, err)

	// Send response trailers
	err = stream.Send(&epb.ProcessingRequest{
		Request: &epb.ProcessingRequest_ResponseTrailers{},
	})
	require.NoError(t, err)

	// Wait a bit for the server to process all requests
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	stream.CloseSend()
	s.GracefulStop()
	mockProcessor.AssertExpectations(t)
}

func TestServeWithOptions(t *testing.T) {
	mockProcessor := new(MockRequestProcessor)
	mockProcessor.On("GetName").Return("test-processor")
	mockProcessor.On("GetOptions").Return(NewDefaultOptions())

	// Use a random port to avoid conflicts
	port := 0 // will assign random available port
	serverOpts := ExtProcServerOptions{
		GracefulShutdownTimeout: 1,
		MaxConcurrentStreams:    10,
	}

	// Start server in a goroutine
	go func() {
		ServeWithOptions(port, serverOpts, mockProcessor)
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Send termination signal
	// Note: In a real test, you would use syscall.SIGTERM,
	// but for testing we'll simulate the shutdown directly
	mockProcessor.AssertExpectations(t)
}

func TestServeWithNilProcessor(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic with nil processor")
		}
	}()

	Serve(0, nil)
}
