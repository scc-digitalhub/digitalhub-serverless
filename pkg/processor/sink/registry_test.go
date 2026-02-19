// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
// SPDX-License-Identifier: Apache-2.0

package sink

import (
	"context"
	"testing"

	"github.com/nuclio/logger"
	"github.com/stretchr/testify/suite"
)

type RegistryTestSuite struct {
	suite.Suite
	registry *Registry
}

func (suite *RegistryTestSuite) SetupTest() {
	suite.registry = NewRegistry()
}

func (suite *RegistryTestSuite) TestRegisterAndGet() {
	factory := &mockFactory{kind: "test"}

	// Register factory
	suite.registry.Register("test", factory)

	// Get factory
	retrieved, err := suite.registry.Get("test")
	suite.NoError(err)
	suite.Equal(factory, retrieved)
}

func (suite *RegistryTestSuite) TestGetNotFound() {
	_, err := suite.registry.Get("nonexistent")
	suite.Error(err)
	suite.Contains(err.Error(), "sink factory not found")
}

func (suite *RegistryTestSuite) TestCreate() {
	factory := &mockFactory{kind: "test"}
	suite.registry.Register("test", factory)

	// Create sink
	sink, err := suite.registry.Create(nil, "test", map[string]interface{}{})
	suite.NoError(err)
	suite.NotNil(sink)
	suite.Equal("test", sink.GetKind())
}

func (suite *RegistryTestSuite) TestGetRegisteredKinds() {
	suite.registry.Register("test1", &mockFactory{kind: "test1"})
	suite.registry.Register("test2", &mockFactory{kind: "test2"})

	kinds := suite.registry.GetRegisteredKinds()
	suite.Len(kinds, 2)
	suite.Contains(kinds, "test1")
	suite.Contains(kinds, "test2")
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}

// Mock implementations

type mockFactory struct {
	kind string
}

func (f *mockFactory) Create(logger logger.Logger, configuration map[string]interface{}) (Sink, error) {
	return &mockSink{kind: f.kind}, nil
}

func (f *mockFactory) GetKind() string {
	return f.kind
}

type mockSink struct {
	kind string
}

func (s *mockSink) Start() error {
	return nil
}

func (s *mockSink) Stop(force bool) error {
	return nil
}

func (s *mockSink) Write(ctx context.Context, data []byte, metadata map[string]interface{}) error {
	return nil
}

func (s *mockSink) GetKind() string {
	return s.kind
}

func (s *mockSink) GetConfig() map[string]interface{} {
	return map[string]interface{}{"kind": s.kind}
}
