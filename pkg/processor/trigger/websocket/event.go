/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package websocket

import (
	"strconv"
	"time"

	"github.com/nuclio/nuclio-sdk-go"
)

// Event wraps a single message received over WebSocket
type Event struct {
	nuclio.AbstractEvent
	body       []byte
	attributes map[string]interface{}
	timestamp  time.Time
}

// GetContentType returns the content type of the WebSocket message
func (e *Event) GetContentType() string {
	return "application/octet-stream"
}

// GetBody returns the WebSocket message data
func (e *Event) GetBody() []byte {
	return e.body
}

// GetHeaderByteSlice returns a header value as a byte slice
func (e *Event) GetHeaderByteSlice(key string) []byte {
	if val, ok := e.attributes[key]; ok {
		if strVal, ok := val.(string); ok {
			return []byte(strVal)
		}
	}
	return nil
}

// GetHeader returns a header value as an interface{}
func (e *Event) GetHeader(key string) interface{} {
	if e.attributes == nil {
		return nil
	}
	return e.attributes[key]
}

// GetHeaders returns all attributes as headers
func (e *Event) GetHeaders() map[string]interface{} {
	return e.attributes
}

// GetHeaderString returns a header value as a string
func (e *Event) GetHeaderString(key string) string {
	return string(e.GetHeaderByteSlice(key))
}

// GetHeaderInt returns a header value as an int
func (e *Event) GetHeaderInt(key string) (int, error) {
	header := e.GetHeader(key)
	if header == nil {
		return 0, nil
	}

	switch typedValue := header.(type) {
	case int:
		return typedValue, nil
	case int64:
		return int(typedValue), nil
	case float64:
		return int(typedValue), nil
	case string:
		intVal, err := strconv.Atoi(typedValue)
		return intVal, err
	default:
		return 0, nil
	}
}

// GetMethod returns "websocket" as the method
func (e *Event) GetMethod() string {
	return "websocket"
}

// GetPath returns empty path (not applicable for WebSocket)
func (e *Event) GetPath() string {
	return ""
}

// GetFieldByteSlice returns an attribute value as a byte slice
func (e *Event) GetFieldByteSlice(key string) []byte {
	return e.GetHeaderByteSlice(key)
}

// GetFieldString returns an attribute value as a string
func (e *Event) GetFieldString(key string) string {
	return e.GetHeaderString(key)
}

// GetFieldInt returns an attribute value as an int
func (e *Event) GetFieldInt(key string) (int, error) {
	return e.GetHeaderInt(key)
}

// GetFields returns all attributes
func (e *Event) GetFields() map[string]interface{} {
	return e.attributes
}

// GetField returns an attribute by key
func (e *Event) GetField(key string) interface{} {
	return e.GetHeader(key)
}

// GetTimestamp returns the packet timestamp
func (e *Event) GetTimestamp() time.Time {
	return e.timestamp
}
