/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package job

import (
	"strconv"
	"time"
)

// Event contains the data for a job event
type Event struct {
	Body       []byte
	Attributes map[string]interface{}
	timestamp  time.Time
}

// GetBody returns the body of the job event
func (e *Event) GetBody() []byte {
	return e.Body
}

// GetBodyString returns the body of the job event as a string
func (e *Event) GetBodyString() string {
	return string(e.Body)
}

// GetPath returns an empty string since job events don't have a path
func (e *Event) GetPath() string {
	return ""
}

// GetURL returns an empty string since job events don't have a URL
func (e *Event) GetURL() string {
	return ""
}

// GetMethod returns an empty string since job events don't have a method
func (e *Event) GetMethod() string {
	return ""
}

// GetShardID returns 0 since job events don't have a shard ID
func (e *Event) GetShardID() int {
	return 0
}

// GetType returns "job" as the type
func (e *Event) GetType() string {
	return "job"
}

// GetTypeVersion returns an empty string since job events don't have a type version
func (e *Event) GetTypeVersion() string {
	return ""
}

// GetVersion returns 0 since job events don't have a version
func (e *Event) GetVersion() int {
	return 0
}

// GetID returns an empty string since job events don't have an ID
func (e *Event) GetID() string {
	return ""
}

// GetTriggerInfo returns a map with trigger information
func (e *Event) GetTriggerInfo() map[string]interface{} {
	return map[string]interface{}{
		"class": "job",
	}
}

// GetHeaders returns nil since job events don't have headers
func (e *Event) GetHeaders() map[string]interface{} {
	return nil
}

// GetTimestamp returns the event timestamp
func (e *Event) GetTimestamp() time.Time {
	return e.timestamp
}

// GetContentType returns an empty string since job events don't have a content type
func (e *Event) GetContentType() string {
	return ""
}

// GetFields returns the event attributes
func (e *Event) GetFields() map[string]interface{} {
	return e.Attributes
}

// GetField returns a specific event attribute
func (e *Event) GetField(key string) (interface{}, error) {
	if e.Attributes == nil {
		return nil, nil
	}
	return e.Attributes[key], nil
}

// GetFieldString returns a specific event attribute as a string
func (e *Event) GetFieldString(key string) (string, error) {
	value, err := e.GetField(key)
	if err != nil {
		return "", err
	}
	if value == nil {
		return "", nil
	}

	switch typedValue := value.(type) {
	case string:
		return typedValue, nil
	case int:
		return strconv.Itoa(typedValue), nil
	case float64:
		return strconv.FormatFloat(typedValue, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(typedValue), nil
	default:
		return "", nil
	}
}
