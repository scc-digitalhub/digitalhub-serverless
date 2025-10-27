/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package extproc

import (
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAllHeadersFromEnvoyHeaderMap(t *testing.T) {
	tests := []struct {
		name      string
		headerMap *corev3.HeaderMap
		want      AllHeaders
		wantErr   bool
	}{
		{
			name: "empty header map",
			headerMap: &corev3.HeaderMap{
				Headers: []*corev3.HeaderValue{},
			},
			want: AllHeaders{
				Headers:    map[string]string{},
				RawHeaders: map[string][]byte{},
			},
		},
		{
			name: "normal string headers",
			headerMap: &corev3.HeaderMap{
				Headers: []*corev3.HeaderValue{
					{Key: "content-type", Value: "application/json"},
					{Key: "x-request-id", Value: "123"},
				},
			},
			want: AllHeaders{
				Headers: map[string]string{
					"content-type": "application/json",
					"x-request-id": "123",
				},
				RawHeaders: map[string][]byte{},
			},
		},
		{
			name: "raw headers",
			headerMap: &corev3.HeaderMap{
				Headers: []*corev3.HeaderValue{
					{Key: "content-type", RawValue: []byte("application/json")},
					{Key: "binary-data", RawValue: []byte{0xFF, 0xFE, 0xFD}},
				},
			},
			want: AllHeaders{
				Headers: map[string]string{},
				RawHeaders: map[string][]byte{
					"content-type": []byte("application/json"),
					"binary-data":  {0xFF, 0xFE, 0xFD},
				},
			},
		},
		{
			name: "mixed headers",
			headerMap: &corev3.HeaderMap{
				Headers: []*corev3.HeaderValue{
					{Key: "content-type", Value: "application/json"},
					{Key: "binary-data", RawValue: []byte{0xFF, 0xFE, 0xFD}},
				},
			},
			want: AllHeaders{
				Headers: map[string]string{
					"content-type": "application/json",
				},
				RawHeaders: map[string][]byte{
					"binary-data": {0xFF, 0xFE, 0xFD},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAllHeadersFromEnvoyHeaderMap(tt.headerMap)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAllHeaders_Stringify(t *testing.T) {
	tests := []struct {
		name      string
		headers   AllHeaders
		want      map[string]string
		hasBinary bool // Used to check binary data base64 encoding
	}{
		{
			name: "empty headers",
			headers: AllHeaders{
				Headers:    map[string]string{},
				RawHeaders: map[string][]byte{},
			},
			want:      map[string]string{},
			hasBinary: false,
		},
		{
			name: "string headers only",
			headers: AllHeaders{
				Headers: map[string]string{
					"content-type": "application/json",
					"x-request-id": "123",
				},
				RawHeaders: map[string][]byte{},
			},
			want: map[string]string{
				"content-type": "application/json",
				"x-request-id": "123",
			},
			hasBinary: false,
		},
		{
			name: "raw headers with valid UTF-8",
			headers: AllHeaders{
				Headers: map[string]string{},
				RawHeaders: map[string][]byte{
					"content-type": []byte("application/json"),
				},
			},
			want: map[string]string{
				"content-type": "application/json",
			},
			hasBinary: false,
		},
		{
			name: "raw headers with invalid UTF-8",
			headers: AllHeaders{
				Headers: map[string]string{},
				RawHeaders: map[string][]byte{
					"binary-data": {0xFF, 0xFE, 0xFD},
				},
			},
			want: map[string]string{
				"binary-data": "/+79", // base64 encoded
			},
			hasBinary: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.headers.Stringify()
			if tt.hasBinary {
				// For binary data, verify the base64 encoded length
				for key, value := range tt.want {
					gotValue, exists := got[key]
					assert.True(t, exists)
					assert.Equal(t, len(value), len(gotValue), "Base64 encoded strings should have the same length")
				}
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestAllHeaders_GetHeaderValue(t *testing.T) {
	headers := AllHeaders{
		Headers: map[string]string{
			"string-header": "value",
		},
		RawHeaders: map[string][]byte{
			"raw-header": []byte("raw-value"),
		},
	}

	tests := []struct {
		name       string
		headerName string
		wantStr    *string
		wantRaw    []byte
		exists     bool
	}{
		{
			name:       "existing string header",
			headerName: "string-header",
			wantStr:    stringPtr("value"),
			wantRaw:    nil,
			exists:     true,
		},
		{
			name:       "existing raw header",
			headerName: "raw-header",
			wantStr:    nil,
			wantRaw:    []byte("raw-value"),
			exists:     true,
		},
		{
			name:       "non-existent header",
			headerName: "not-found",
			wantStr:    nil,
			wantRaw:    nil,
			exists:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str, raw, exists := headers.GetHeaderValue(tt.headerName)
			assert.Equal(t, tt.exists, exists)
			if tt.wantStr != nil {
				assert.Equal(t, *tt.wantStr, *str)
			} else {
				assert.Nil(t, str)
			}
			assert.Equal(t, tt.wantRaw, raw)
		})
	}
}

func TestAllHeaders_GetHeaderValueAsString(t *testing.T) {
	headers := AllHeaders{
		Headers: map[string]string{
			"string-header": "value",
		},
		RawHeaders: map[string][]byte{
			"raw-utf8":   []byte("raw-value"),
			"raw-binary": {0xFF, 0xFE, 0xFD},
		},
	}

	tests := []struct {
		name       string
		headerName string
		want       string
		wantErr    bool
		isBinary   bool
	}{
		{
			name:       "existing string header",
			headerName: "string-header",
			want:       "value",
			wantErr:    false,
			isBinary:   false,
		},
		{
			name:       "raw utf8 header",
			headerName: "raw-utf8",
			want:       "raw-value",
			wantErr:    false,
			isBinary:   false,
		},
		{
			name:       "raw binary header",
			headerName: "raw-binary",
			want:       "/v79", // base64 encoded [0xFF, 0xFE, 0xFD]
			wantErr:    false,
			isBinary:   true,
		},
		{
			name:       "non-existent header",
			headerName: "missing",
			want:       "",
			wantErr:    true,
			isBinary:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := headers.GetHeaderValueAsString(tt.headerName)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.isBinary {
				// For binary data, verify base64 encoded string length
				assert.Equal(t, len(tt.want), len(got), "Base64 encoded strings should have the same length")
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestAllHeaders_DropHeaderNamed(t *testing.T) {
	tests := []struct {
		name       string
		headers    AllHeaders
		headerName string
		want       bool
		wantAfter  AllHeaders
	}{
		{
			name: "drop existing string header",
			headers: AllHeaders{
				Headers:    map[string]string{"to-drop": "value", "keep": "value"},
				RawHeaders: map[string][]byte{},
			},
			headerName: "to-drop",
			want:       true,
			wantAfter: AllHeaders{
				Headers:    map[string]string{"keep": "value"},
				RawHeaders: map[string][]byte{},
			},
		},
		{
			name: "drop existing raw header",
			headers: AllHeaders{
				Headers: map[string]string{},
				RawHeaders: map[string][]byte{
					"to-drop": []byte("value"),
					"keep":    []byte("value"),
				},
			},
			headerName: "to-drop",
			want:       true,
			wantAfter: AllHeaders{
				Headers: map[string]string{},
				RawHeaders: map[string][]byte{
					"keep": []byte("value"),
				},
			},
		},
		{
			name: "drop non-existent header",
			headers: AllHeaders{
				Headers:    map[string]string{"keep": "value"},
				RawHeaders: map[string][]byte{"keep": []byte("value")},
			},
			headerName: "not-found",
			want:       false,
			wantAfter: AllHeaders{
				Headers:    map[string]string{"keep": "value"},
				RawHeaders: map[string][]byte{"keep": []byte("value")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.headers.DropHeaderNamed(tt.headerName)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantAfter, tt.headers)
		})
	}
}

func TestAllHeaders_DropHeadersNamed(t *testing.T) {
	headers := AllHeaders{
		Headers: map[string]string{
			"drop1": "value1",
			"keep1": "value2",
		},
		RawHeaders: map[string][]byte{
			"drop2": []byte("value3"),
			"keep2": []byte("value4"),
		},
	}

	expected := AllHeaders{
		Headers: map[string]string{
			"keep1": "value2",
		},
		RawHeaders: map[string][]byte{
			"keep2": []byte("value4"),
		},
	}

	headers.DropHeadersNamed([]string{"drop1", "drop2", "non-existent"})
	assert.Equal(t, expected, headers)
}

func TestAllHeaders_FilterHeaders(t *testing.T) {
	headers := AllHeaders{
		Headers: map[string]string{
			"x-test1": "value1",
			"y-test1": "value2",
		},
		RawHeaders: map[string][]byte{
			"x-test2": []byte("value3"),
			"y-test2": []byte("value4"),
		},
	}

	filterX := func(name string) bool {
		return name[0] == 'x'
	}

	expected := AllHeaders{
		Headers: map[string]string{
			"y-test1": "value2",
		},
		RawHeaders: map[string][]byte{
			"y-test2": []byte("value4"),
		},
	}

	headers.FilterHeaders(filterX)
	assert.Equal(t, expected, headers)
}

func TestAllHeaders_DropHeadersNamedStartingWith(t *testing.T) {
	headers := AllHeaders{
		Headers: map[string]string{
			"x-test1": "value1",
			"y-test1": "value2",
		},
		RawHeaders: map[string][]byte{
			"x-test2": []byte("value3"),
			"y-test2": []byte("value4"),
		},
	}

	expected := AllHeaders{
		Headers: map[string]string{
			"y-test1": "value2",
		},
		RawHeaders: map[string][]byte{
			"y-test2": []byte("value4"),
		},
	}

	headers.DropHeadersNamedStartingWith("x-")
	assert.Equal(t, expected, headers)
}

func TestAllHeaders_DropHeadersNamedEndingWith(t *testing.T) {
	headers := AllHeaders{
		Headers: map[string]string{
			"test1-x": "value1",
			"test1-y": "value2",
		},
		RawHeaders: map[string][]byte{
			"test2-x": []byte("value3"),
			"test2-y": []byte("value4"),
		},
	}

	expected := AllHeaders{
		Headers: map[string]string{
			"test1-y": "value2",
		},
		RawHeaders: map[string][]byte{
			"test2-y": []byte("value4"),
		},
	}

	headers.DropHeadersNamedEndingWith("-x")
	assert.Equal(t, expected, headers)
}

func TestAllHeaders_Clone(t *testing.T) {
	original := AllHeaders{
		Headers: map[string]string{
			"string-header": "value1",
		},
		RawHeaders: map[string][]byte{
			"raw-header": []byte("value2"),
		},
	}

	clone := original.Clone()

	// Check that values are equal
	assert.Equal(t, original, *clone)

	// Modify clone and check that original is unchanged
	clone.Headers["new-header"] = "new-value"
	clone.RawHeaders["new-raw"] = []byte("new-value")

	assert.NotEqual(t, original, *clone)
	assert.NotContains(t, original.Headers, "new-header")
	assert.NotContains(t, original.RawHeaders, "new-raw")
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
