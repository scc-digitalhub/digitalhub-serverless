/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package mjpeg

import (
	"time"

	"github.com/nuclio/nuclio-sdk-go"
)

// Event contains the data for a MJPEG frame event
type Event struct {
	nuclio.AbstractEvent
	body      []byte
	timestamp time.Time
	frameNum  int64
	url       string
}

// GetBody returns the frame data (JPEG image bytes)
func (e *Event) GetBody() []byte {
	return e.body
}

// GetBodyString returns the frame data as a string (not recommended for binary data)
func (e *Event) GetBodyString() string {
	return string(e.body)
}

// GetBodyObject returns nil since MJPEG events don't have a body object
func (e *Event) GetBodyObject() interface{} {
	return nil
}

// GetPath returns the URL path of the MJPEG stream
func (e *Event) GetPath() string {
	return e.url
}

// GetURL returns the full URL of the MJPEG stream
func (e *Event) GetURL() string {
	return e.url
}

// GetMethod returns an empty string since MJPEG events don't have a method
func (e *Event) GetMethod() string {
	return ""
}

// GetShardID returns 0 since MJPEG events don't have a shard ID
func (e *Event) GetShardID() int {
	return 0
}

// GetType returns "mjpeg" as the type
func (e *Event) GetType() string {
	return "mjpeg"
}

// GetTypeVersion returns an empty string since MJPEG events don't have a type version
func (e *Event) GetTypeVersion() string {
	return ""
}

// GetHeaders returns nil since MJPEG events don't have headers
func (e *Event) GetHeaders() map[string]interface{} {
	return nil
}

// GetHeader returns nil since MJPEG events don't have headers
func (e *Event) GetHeader(key string) interface{} {
	return nil
}

// GetHeaderByteSlice returns nil since MJPEG events don't have headers
func (e *Event) GetHeaderByteSlice(key string) []byte {
	return nil
}

// GetHeaderString returns empty string since MJPEG events don't have headers
func (e *Event) GetHeaderString(key string) string {
	return ""
}

// GetHeaderInt returns 0 since MJPEG events don't have headers
func (e *Event) GetHeaderInt(key string) (int, error) {
	return 0, nil
}

// GetTimestamp returns the event timestamp
func (e *Event) GetTimestamp() time.Time {
	return e.timestamp
}

// GetContentType returns the content type for JPEG images
func (e *Event) GetContentType() string {
	return "image/jpeg"
}

// GetFields returns the event fields
func (e *Event) GetFields() map[string]interface{} {
	return map[string]interface{}{
		"frame_num": e.frameNum,
		"url":       e.url,
		"timestamp": e.timestamp,
	}
}

// GetField returns a specific event field
func (e *Event) GetField(key string) interface{} {
	fields := e.GetFields()
	return fields[key]
}

// GetFieldString returns a specific event field as a string
func (e *Event) GetFieldString(key string) string {
	value := e.GetField(key)
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

// GetFieldByteSlice returns an event field as a byte slice
func (e *Event) GetFieldByteSlice(key string) []byte {
	value := e.GetField(key)
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case string:
		return []byte(v)
	case []byte:
		return v
	default:
		return nil
	}
}

// GetFieldInt returns an event field as an int
func (e *Event) GetFieldInt(key string) (int, error) {
	value := e.GetField(key)
	if value == nil {
		return 0, nil
	}

	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, nil
	}
}

// GetSize returns the size of the frame data
func (e *Event) GetSize() int {
	return len(e.body)
}
