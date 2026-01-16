/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package websocket

import (
	"testing"
	"time"
)

func TestDataProcessorDiscrete_EmitsEventAfterPush(t *testing.T) {
	dp := NewDataProcessorDiscrete(10 * time.Millisecond)
	dp.Start()
	defer dp.Stop()

	input := []byte("hello")

	dp.Push(input)

	select {
	case ev := <-dp.Output():
		if string(ev.body) != "hello" {
			t.Fatalf("expected 'hello', got '%s'", string(ev.body))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestDataProcessorDiscrete_EmitsOnlyOncePerPush(t *testing.T) {
	dp := NewDataProcessorDiscrete(10 * time.Millisecond)
	dp.Start()
	defer dp.Stop()

	dp.Push([]byte("msg"))

	// first emission
	select {
	case <-dp.Output():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for first event")
	}

	// should not emit again without new Push
	select {
	case ev := <-dp.Output():
		t.Fatalf("unexpected second event: %v", ev)
	case <-time.After(50 * time.Millisecond):
		// success: no event
	}
}

func TestDataProcessorDiscrete_OverwritesOldData(t *testing.T) {
	dp := NewDataProcessorDiscrete(10 * time.Millisecond)
	dp.Start()
	defer dp.Stop()

	dp.Push([]byte("old"))
	dp.Push([]byte("new"))

	select {
	case ev := <-dp.Output():
		if string(ev.body) != "new" {
			t.Fatalf("expected 'new', got '%s'", string(ev.body))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}
