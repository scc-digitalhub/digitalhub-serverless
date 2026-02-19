// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
// SPDX-License-Identifier: Apache-2.0

package sink

import (
	"fmt"
	"sync"

	"github.com/nuclio/logger"
)

// Registry manages sink factories
type Registry struct {
	lock      sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates a new registry
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Register registers a sink factory
func (r *Registry) Register(kind string, factory Factory) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.factories[kind] = factory
}

// Get retrieves a factory by kind
func (r *Registry) Get(kind string) (Factory, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	factory, ok := r.factories[kind]
	if !ok {
		return nil, fmt.Errorf("sink factory not found: %s", kind)
	}

	return factory, nil
}

// Create creates a new sink instance
func (r *Registry) Create(logger logger.Logger, kind string, configuration map[string]interface{}) (Sink, error) {
	factory, err := r.Get(kind)
	if err != nil {
		return nil, err
	}

	return factory.Create(logger, configuration)
}

// GetRegisteredKinds returns a list of all registered sink kinds
func (r *Registry) GetRegisteredKinds() []string {
	r.lock.RLock()
	defer r.lock.RUnlock()

	kinds := make([]string, 0, len(r.factories))
	for kind := range r.factories {
		kinds = append(kinds, kind)
	}

	return kinds
}

// RegistrySingleton is the global sink registry
var RegistrySingleton = NewRegistry()
