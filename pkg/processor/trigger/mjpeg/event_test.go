/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package mjpeg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEvent(t *testing.T) {
	now := time.Now()
	frameData := []byte{0xFF, 0xD8, 0xFF, 0xE0} // JPEG header

	event := &Event{
		body:      frameData,
		timestamp: now,
		frameNum:  42,
		url:       "http://example.com/stream.mjpg",
	}

	t.Run("GetBody", func(t *testing.T) {
		assert.Equal(t, frameData, event.GetBody())
	})

	t.Run("GetBodyString", func(t *testing.T) {
		assert.Equal(t, string(frameData), event.GetBodyString())
	})

	t.Run("GetBodyObject", func(t *testing.T) {
		assert.Nil(t, event.GetBodyObject())
	})

	t.Run("GetPath", func(t *testing.T) {
		assert.Equal(t, "http://example.com/stream.mjpg", event.GetPath())
	})

	t.Run("GetURL", func(t *testing.T) {
		assert.Equal(t, "http://example.com/stream.mjpg", event.GetURL())
	})

	t.Run("GetMethod", func(t *testing.T) {
		assert.Equal(t, "", event.GetMethod())
	})

	t.Run("GetShardID", func(t *testing.T) {
		assert.Equal(t, 0, event.GetShardID())
	})

	t.Run("GetType", func(t *testing.T) {
		assert.Equal(t, "mjpeg", event.GetType())
	})

	t.Run("GetTypeVersion", func(t *testing.T) {
		assert.Equal(t, "", event.GetTypeVersion())
	})

	t.Run("GetTimestamp", func(t *testing.T) {
		assert.Equal(t, now, event.GetTimestamp())
	})

	t.Run("GetContentType", func(t *testing.T) {
		assert.Equal(t, "image/jpeg", event.GetContentType())
	})

	t.Run("GetSize", func(t *testing.T) {
		assert.Equal(t, len(frameData), event.GetSize())
	})

	t.Run("GetHeaders", func(t *testing.T) {
		assert.Nil(t, event.GetHeaders())
	})

	t.Run("GetHeader", func(t *testing.T) {
		assert.Nil(t, event.GetHeader("any-key"))
	})

	t.Run("GetHeaderByteSlice", func(t *testing.T) {
		assert.Nil(t, event.GetHeaderByteSlice("any-key"))
	})

	t.Run("GetHeaderString", func(t *testing.T) {
		assert.Equal(t, "", event.GetHeaderString("any-key"))
	})

	t.Run("GetHeaderInt", func(t *testing.T) {
		val, err := event.GetHeaderInt("any-key")
		assert.NoError(t, err)
		assert.Equal(t, 0, val)
	})

	t.Run("GetFields", func(t *testing.T) {
		fields := event.GetFields()
		assert.NotNil(t, fields)
		assert.Equal(t, int64(42), fields["frame_num"])
		assert.Equal(t, "http://example.com/stream.mjpg", fields["url"])
		assert.Equal(t, now, fields["timestamp"])
	})

	t.Run("GetField", func(t *testing.T) {
		frameNum := event.GetField("frame_num")
		assert.Equal(t, int64(42), frameNum)

		url := event.GetField("url")
		assert.Equal(t, "http://example.com/stream.mjpg", url)

		timestamp := event.GetField("timestamp")
		assert.Equal(t, now, timestamp)

		nonexistent := event.GetField("nonexistent")
		assert.Nil(t, nonexistent)
	})

	t.Run("GetFieldString", func(t *testing.T) {
		url := event.GetFieldString("url")
		assert.Equal(t, "http://example.com/stream.mjpg", url)

		// Non-string field should return empty string
		frameNum := event.GetFieldString("frame_num")
		assert.Equal(t, "", frameNum)

		// Nonexistent field should return empty string
		nonexistent := event.GetFieldString("nonexistent")
		assert.Equal(t, "", nonexistent)
	})

	t.Run("GetFieldByteSlice", func(t *testing.T) {
		url := event.GetFieldByteSlice("url")
		assert.Equal(t, []byte("http://example.com/stream.mjpg"), url)

		// Non-string/byte field should return nil
		frameNum := event.GetFieldByteSlice("frame_num")
		assert.Nil(t, frameNum)

		// Nonexistent field should return nil
		nonexistent := event.GetFieldByteSlice("nonexistent")
		assert.Nil(t, nonexistent)
	})

	t.Run("GetFieldInt", func(t *testing.T) {
		frameNum, err := event.GetFieldInt("frame_num")
		assert.NoError(t, err)
		assert.Equal(t, 42, frameNum)

		// Non-int field should return 0
		url, err := event.GetFieldInt("url")
		assert.NoError(t, err)
		assert.Equal(t, 0, url)

		// Nonexistent field should return 0
		nonexistent, err := event.GetFieldInt("nonexistent")
		assert.NoError(t, err)
		assert.Equal(t, 0, nonexistent)
	})
}

func TestEventWithDifferentData(t *testing.T) {
	t.Run("LargeFrame", func(t *testing.T) {
		largeData := make([]byte, 1024*1024) // 1MB
		event := &Event{
			body:      largeData,
			timestamp: time.Now(),
			frameNum:  100,
			url:       "http://camera/stream",
		}

		assert.Equal(t, largeData, event.GetBody())
		assert.Equal(t, 1024*1024, event.GetSize())
	})

	t.Run("EmptyFrame", func(t *testing.T) {
		event := &Event{
			body:      []byte{},
			timestamp: time.Now(),
			frameNum:  0,
			url:       "http://camera/stream",
		}

		assert.Equal(t, []byte{}, event.GetBody())
		assert.Equal(t, 0, event.GetSize())
	})

	t.Run("NilFrame", func(t *testing.T) {
		event := &Event{
			body:      nil,
			timestamp: time.Now(),
			frameNum:  1,
			url:       "http://camera/stream",
		}

		assert.Nil(t, event.GetBody())
		assert.Equal(t, 0, event.GetSize())
	})
}
