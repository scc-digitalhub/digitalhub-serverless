/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package job

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobEvent(t *testing.T) {
	tests := []struct {
		name          string
		event         Event
		expectedBody  []byte
		expectedAttrs map[string]interface{}
	}{
		{
			name: "basic event",
			event: Event{
				Body:       []byte("test body"),
				Attributes: map[string]interface{}{"key": "value"},
			},
			expectedBody:  []byte("test body"),
			expectedAttrs: map[string]interface{}{"key": "value"},
		},
		{
			name: "nil body",
			event: Event{
				Body:       nil,
				Attributes: map[string]interface{}{"key": "value"},
			},
			expectedBody:  nil,
			expectedAttrs: map[string]interface{}{"key": "value"},
		},
		{
			name: "nil attributes",
			event: Event{
				Body:       []byte("test body"),
				Attributes: nil,
			},
			expectedBody:  []byte("test body"),
			expectedAttrs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test GetBody method
			body := tt.event.GetBody()
			assert.Equal(t, tt.expectedBody, body)

			// Test GetBodyString method
			if tt.expectedBody != nil {
				bodyStr := tt.event.GetBodyString()
				assert.Equal(t, string(tt.expectedBody), bodyStr)
			}

			// Test GetPath method (should be empty for job events)
			assert.Equal(t, "", tt.event.GetPath())

			// Test GetURL method (should be empty for job events)
			assert.Equal(t, "", tt.event.GetURL())

			// Test GetMethod method (should be empty for job events)
			assert.Equal(t, "", tt.event.GetMethod())

			// Test GetShardID method (should be 0 for job events)
			assert.Equal(t, 0, tt.event.GetShardID())

			// Test GetType method (should be "job" for job events)
			assert.Equal(t, "job", tt.event.GetType())

			// Test GetTypeVersion method (should be "" for job events)
			assert.Equal(t, "", tt.event.GetTypeVersion())

			// Test GetVersion method (should be 0 for job events)
			assert.Equal(t, 0, tt.event.GetVersion())

			// Test GetID method (should be "" for job events)
			assert.Equal(t, "", tt.event.GetID())

			// Test GetTriggerInfo method
			assert.NotNil(t, tt.event.GetTriggerInfo())

			// Test GetHeaders method (should be empty for job events)
			assert.Empty(t, tt.event.GetHeaders())

			// Test GetTimestamp method
			assert.NotNil(t, tt.event.GetTimestamp())

			// Test GetContentType method (should be empty for job events)
			assert.Equal(t, "", tt.event.GetContentType())

			// Test GetFields method
			fields := tt.event.GetFields()
			assert.Equal(t, tt.expectedAttrs, fields)

			// Test field getters
			if tt.expectedAttrs != nil {
				for k, v := range tt.expectedAttrs {
					if str, ok := v.(string); ok {
						val, err := tt.event.GetFieldString(k)
						require.NoError(t, err)
						assert.Equal(t, str, val)
					}

					val, err := tt.event.GetField(k)
					require.NoError(t, err)
					assert.Equal(t, v, val)
				}
			}
		})
	}
}
