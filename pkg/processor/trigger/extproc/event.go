/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package extproc

import (
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio-sdk-go"
)

// allows accessing fasthttp.RequestCtx as a event.Sync
type Event struct {
	nuclio.AbstractEvent
	ctx *RequestContext
}

// GetContentType returns the content type of the body
func (e *Event) GetContentType() string {
	return e.GetHeaderString("Content-Type")
}

// GetBody returns the body of the event
func (e *Event) GetBody() []byte {
	return e.ctx.CurrentBodyBytes()
}

// GetHeaderByteSlice returns the header by name as a byte slice
func (e *Event) GetHeaderByteSlice(key string) []byte {

	// TODO: consider lifetime of the header. User may not keep a reference
	_, value, _ := e.ctx.AllHeaders.GetHeaderValue(key)
	return value
}

// GetHeader returns the header by name as an interface{}
func (e *Event) GetHeader(key string) interface{} {
	return e.GetHeaderByteSlice(key)
}

// GetHeaders loads all headers into a map of string / interface{}
func (e *Event) GetHeaders() map[string]interface{} {
	headers := make(map[string]interface{})
	for key, value := range e.ctx.AllHeaders.Headers {
		headers[string(key)] = strings.Join(value, ",")
	}
	return headers
}

// GetHeaderString returns the header by name as a string
func (e *Event) GetHeaderString(key string) string {
	return string(e.GetHeaderByteSlice(key))
}

// GetPath returns the method of the event, if applicable
func (e *Event) GetMethod() string {
	return string(e.ctx.Method)
}

// GetPath returns the path of the event
func (e *Event) GetPath() string {
	return string(e.ctx.Path)
}

// GetFieldByteSlice returns the field by name as a byte slice
func (e *Event) GetFieldByteSlice(key string) []byte {
	return []byte(strings.Join(e.ctx.Query[key], ","))
}

// GetFieldString returns the field by name as a string
func (e *Event) GetFieldString(key string) string {
	return string(e.GetFieldByteSlice(key))
}

func (e *Event) GetFieldInt(key string) (int, error) {
	val := e.ctx.Query[key]
	if len(val) == 0 {
		return 0, nil
	}
	return strconv.Atoi(val[0])
}

// GetFields loads all fields into a map of string / interface{}
func (e *Event) GetFields() map[string]interface{} {
	fields := make(map[string]interface{})
	for key, value := range e.ctx.Query {
		fields[string(key)] = strings.Join(value, ",")
	}

	return fields
}

// GetTimestamp returns when the event originated
func (e *Event) GetTimestamp() time.Time {
	return e.ctx.Started
}
