/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package mjpeg

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBoundary(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    string
	}{
		{
			name:        "Standard boundary",
			contentType: "multipart/x-mixed-replace;boundary=myboundary",
			expected:    "myboundary",
		},
		{
			name:        "Boundary with spaces",
			contentType: "multipart/x-mixed-replace; boundary = myboundary",
			expected:    "myboundary",
		},
		{
			name:        "No boundary",
			contentType: "multipart/x-mixed-replace",
			expected:    "",
		},
		{
			name:        "Empty content type",
			contentType: "",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal mjpeg instance just for testing helper methods
			m := &mjpeg{}
			result := m.extractBoundary(tt.contentType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReadHeaders(t *testing.T) {
	headerData := "Content-Type: image/jpeg\r\nContent-Length: 1234\r\n\r\n"
	reader := bufio.NewReader(strings.NewReader(headerData))

	m := &mjpeg{}
	headers, err := m.readHeaders(reader)
	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "image/jpeg", headers["Content-Type"])
	assert.Equal(t, "1234", headers["Content-Length"])
}

func TestGetContentLength(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected int
	}{
		{
			name: "Standard Content-Length",
			headers: map[string]string{
				"Content-Length": "1234",
			},
			expected: 1234,
		},
		{
			name: "Lowercase content-length",
			headers: map[string]string{
				"content-length": "5678",
			},
			expected: 5678,
		},
		{
			name:     "Missing Content-Length",
			headers:  map[string]string{},
			expected: 0,
		},
		{
			name: "Invalid Content-Length",
			headers: map[string]string{
				"Content-Length": "invalid",
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mjpeg{}
			result := m.getContentLength(tt.headers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReadUntil(t *testing.T) {
	testData := "some data\nbefore delimiter\n--boundary\nafter delimiter"
	reader := bufio.NewReader(strings.NewReader(testData))

	m := &mjpeg{}
	result, err := m.readUntil(reader, []byte("--boundary"))
	require.NoError(t, err)
	assert.True(t, bytes.Contains(result, []byte("--boundary")))
}

func TestGetConfig(t *testing.T) {
	m := &mjpeg{
		configuration: &Configuration{
			URL:              "http://example.com/stream.mjpg",
			ProcessingFactor: 5,
		},
	}

	configMap := m.GetConfig()
	require.NotNil(t, configMap)
	assert.Contains(t, configMap, "URL")
	assert.Contains(t, configMap, "ProcessingFactor")
}
