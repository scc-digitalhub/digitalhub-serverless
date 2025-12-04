package extproc

import (
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBodyTypeFromHeaders(t *testing.T) {
	tests := []struct {
		name     string
		headers  AllHeaders
		expected BodyType
	}{
		{
			name: "empty headers",
			headers: AllHeaders{
				Headers:    map[string]string{},
				RawHeaders: map[string][]byte{},
			},
			expected: BodyType{},
		},
		{
			name: "content type only",
			headers: AllHeaders{
				Headers: map[string]string{
					"content-type": "application/json",
				},
			},
			expected: BodyType{
				ContentType: "application/json",
			},
		},
		{
			name: "all headers",
			headers: AllHeaders{
				Headers: map[string]string{
					"content-type":      "application/json",
					"content-encoding":  "gzip",
					"transfer-encoding": "chunked",
				},
			},
			expected: BodyType{
				ContentType:      "application/json",
				ContentEncoding:  "gzip",
				TransferEncoding: "chunked",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewBodyTypeFromHeaders(&tt.headers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBodyType_IsCompressed(t *testing.T) {
	tests := []struct {
		name     string
		bodyType BodyType
		expected bool
	}{
		{
			name:     "no encoding",
			bodyType: BodyType{},
			expected: false,
		},
		{
			name: "with content encoding",
			bodyType: BodyType{
				ContentEncoding: "gzip",
			},
			expected: true,
		},
		{
			name: "with transfer encoding (not chunked)",
			bodyType: BodyType{
				TransferEncoding: "gzip",
			},
			expected: true,
		},
		{
			name: "with chunked transfer encoding",
			bodyType: BodyType{
				TransferEncoding: "chunked",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.bodyType.IsCompressed()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEncodedBody_AppendChunk(t *testing.T) {
	tests := []struct {
		name        string
		initialBody *EncodedBody
		chunk       []byte
		maxSize     int64
		expectError bool
	}{
		{
			name: "append to empty body",
			initialBody: &EncodedBody{
				Value:   []byte{},
				MaxSize: -1,
			},
			chunk:       []byte("test"),
			expectError: false,
		},
		{
			name: "append nil chunk",
			initialBody: &EncodedBody{
				Value:   []byte("existing"),
				MaxSize: -1,
			},
			chunk:       nil,
			expectError: false,
		},
		{
			name: "exceed max size",
			initialBody: &EncodedBody{
				Value:   []byte("existing"),
				MaxSize: 10,
			},
			chunk:       []byte("this will exceed max size"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.initialBody.AppendChunk(tt.chunk)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.chunk != nil {
					assert.Equal(t, tt.chunk, tt.initialBody.Value)
				}
			}
		})
	}
}

func TestEncodedBody_DecompressBody(t *testing.T) {
	// Helper function to create gzipped data
	createGzippedData := func(data []byte) []byte {
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		writer.Write(data)
		writer.Close()
		return buf.Bytes()
	}

	tests := []struct {
		name        string
		body        *EncodedBody
		expectError bool
	}{
		{
			name: "not compressed",
			body: &EncodedBody{
				Type:     BodyType{},
				Value:    []byte("test"),
				Complete: true,
			},
			expectError: false,
		},
		{
			name: "incomplete body",
			body: &EncodedBody{
				Type: BodyType{
					ContentEncoding: "gzip",
				},
				Value:    createGzippedData([]byte("test")),
				Complete: false,
			},
			expectError: true,
		},
		{
			name: "valid gzip compression",
			body: &EncodedBody{
				Type: BodyType{
					ContentEncoding: "gzip",
				},
				Value:    createGzippedData([]byte("test")),
				Complete: true,
			},
			expectError: false,
		},
		{
			name: "invalid gzip data",
			body: &EncodedBody{
				Type: BodyType{
					ContentEncoding: "gzip",
				},
				Value:    []byte("not gzipped"),
				Complete: true,
			},
			expectError: true,
		},
		{
			name: "unsupported encoding",
			body: &EncodedBody{
				Type: BodyType{
					ContentEncoding: "br", // brotli compression
				},
				Value:    []byte("test"),
				Complete: true,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.body.DecompressBody()
			if tt.expectError {
				assert.Error(t, err)
				assert.False(t, tt.body.Decompressed)
			} else {
				assert.NoError(t, err)
				assert.True(t, tt.body.Decompressed)
			}
		})
	}
}

func TestNewEncodedBodyFromHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers *AllHeaders
		want    *EncodedBody
	}{
		{
			name: "empty headers",
			headers: &AllHeaders{
				Headers:    map[string]string{},
				RawHeaders: map[string][]byte{},
			},
			want: &EncodedBody{
				Type:         BodyType{},
				Value:        []byte{},
				MaxSize:      -1,
				Decompressed: true,
			},
		},
		{
			name: "with compression",
			headers: &AllHeaders{
				Headers: map[string]string{
					"content-encoding": "gzip",
				},
			},
			want: &EncodedBody{
				Type: BodyType{
					ContentEncoding: "gzip",
				},
				Value:        []byte{},
				MaxSize:      -1,
				Decompressed: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewEncodedBodyFromHeaders(tt.headers)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEncodedBody_CurrentContentLength(t *testing.T) {
	tests := []struct {
		name string
		body *EncodedBody
		want uint32
	}{
		{
			name: "empty body",
			body: &EncodedBody{
				Value: []byte{},
			},
			want: 0,
		},
		{
			name: "non-empty body",
			body: &EncodedBody{
				Value: []byte("test data"),
			},
			want: 9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.body.CurrentContentLength()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_gUnzipData(t *testing.T) {
	original := []byte("test data for compression")

	// Create gzipped data
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(original)
	require.NoError(t, err)
	writer.Close()
	compressed := buf.Bytes()

	// Test unzipping
	unzipped, err := gUnzipData(compressed)
	require.NoError(t, err)
	assert.Equal(t, original, unzipped)

	// Test with invalid data
	_, err = gUnzipData([]byte("not gzipped"))
	assert.Error(t, err)
}
