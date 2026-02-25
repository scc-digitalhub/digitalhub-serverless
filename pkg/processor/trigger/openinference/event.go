/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package openinference

import (
	"time"

	"github.com/nuclio/nuclio-sdk-go"
)

// Event wraps an OpenInference inference request
type Event struct {
	nuclio.AbstractEvent
	body         []byte
	headers      map[string]any
	timestamp    time.Time
	modelName    string
	modelVersion string
	parameters   map[string]any
}

// triggerInfo implements nuclio.TriggerInfoProvider
type triggerInfo struct {
	class        string
	kind         string
	modelName    string
	modelVersion string
}

// GetClass returns the trigger class
func (ti *triggerInfo) GetClass() string {
	return ti.class
}

// GetKind returns the trigger kind
func (ti *triggerInfo) GetKind() string {
	return ti.kind
}

// GetName returns the trigger name
func (ti *triggerInfo) GetName() string {
	return ti.kind
}

// GetContentType returns the content type
func (e *Event) GetContentType() string {
	return "application/json"
}

// GetBody returns the request body
func (e *Event) GetBody() []byte {
	return e.body
}

// GetHeaderByteSlice returns a header value as a byte slice
func (e *Event) GetHeaderByteSlice(key string) []byte {
	if val, ok := e.headers[key]; ok {
		if strVal, ok := val.(string); ok {
			return []byte(strVal)
		}
	}
	return nil
}

// GetHeader returns a header value as an any
func (e *Event) GetHeader(key string) any {
	if e.headers == nil {
		return nil
	}
	return e.headers[key]
}

// GetHeaders returns all headers
func (e *Event) GetHeaders() map[string]any {
	return e.headers
}

// GetTimestamp returns the event timestamp
func (e *Event) GetTimestamp() time.Time {
	return e.timestamp
}

// GetPath returns an empty string
func (e *Event) GetPath() string {
	return "/v2/models/" + e.modelName + "/infer"
}

// GetURL returns an empty string
func (e *Event) GetURL() string {
	return ""
}

// GetMethod returns the method
func (e *Event) GetMethod() string {
	return "POST"
}

// GetFieldByteSlice returns a field value as a byte slice
func (e *Event) GetFieldByteSlice(key string) []byte {
	if val, ok := e.parameters[key]; ok {
		if strVal, ok := val.(string); ok {
			return []byte(strVal)
		}
	}
	return nil
}

// GetFieldString returns a field value as a string
func (e *Event) GetFieldString(key string) string {
	if val, ok := e.parameters[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return ""
}

// GetFieldInt returns a field value as an int
func (e *Event) GetFieldInt(key string) (int, error) {
	if val, ok := e.parameters[key]; ok {
		if intVal, ok := val.(int); ok {
			return intVal, nil
		}
	}
	return 0, nil
}

// GetTriggerInfo returns trigger information
func (e *Event) GetTriggerInfo() nuclio.TriggerInfoProvider {
	return &triggerInfo{
		class:        "sync",
		kind:         "openinference",
		modelName:    e.modelName,
		modelVersion: e.modelVersion,
	}
}
