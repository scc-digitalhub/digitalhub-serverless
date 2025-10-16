/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package job

import (
	"time"

	"github.com/nuclio/nuclio/pkg/processor/worker"
)

type mockWorkerAllocator struct{}

func newMockWorkerAllocator() (worker.Allocator, error) {
	return &mockWorkerAllocator{}, nil
}

// Allocate implements worker.Allocator
func (m *mockWorkerAllocator) Allocate(timeout time.Duration) (*worker.Worker, error) {
	// Return a mock worker that does nothing
	return &worker.Worker{}, nil
}

// Release implements worker.Allocator
func (m *mockWorkerAllocator) Release(*worker.Worker) {
	// Do nothing
}

// Stop implements worker.Allocator
func (m *mockWorkerAllocator) Stop() {
	// Do nothing
}

// GetNumWorkers implements worker.Allocator
func (m *mockWorkerAllocator) GetNumWorkers() int {
	return 1
}

// GetWorkers implements worker.Allocator
func (m *mockWorkerAllocator) GetWorkers() []*worker.Worker {
	return []*worker.Worker{
		{},
	}
}

// GetNumWorkersAvailable implements worker.Allocator
func (m *mockWorkerAllocator) GetNumWorkersAvailable() int {
	return 1
}

// GetStatistics implements worker.Allocator
func (m *mockWorkerAllocator) GetStatistics() *worker.AllocatorStatistics {
	return &worker.AllocatorStatistics{}
}

// IsTerminated implements worker.Allocator
func (m *mockWorkerAllocator) IsTerminated() bool {
	return false
}

// Shareable implements worker.Allocator
func (m *mockWorkerAllocator) Shareable() bool {
	return false
}

// SignalContinue implements worker.Allocator
func (m *mockWorkerAllocator) SignalContinue() error {
	return nil
}

// SignalDraining implements worker.Allocator
func (m *mockWorkerAllocator) SignalDraining() error {
	return nil
}

// SignalTermination implements worker.Allocator
func (m *mockWorkerAllocator) SignalTermination() error {
	return nil
}
