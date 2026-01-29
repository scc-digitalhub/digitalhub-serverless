package websocket

import (
	"testing"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type WebsocketFactoryTestSuite struct {
	suite.Suite
	logger                logger.Logger
	namedWorkerAllocators *worker.AllocatorSyncMap
}

func (suite *WebsocketFactoryTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("factory-test")
	suite.Require().NoError(err)

	suite.namedWorkerAllocators = worker.NewAllocatorSyncMap()
	allocator, _ := newMockWorkerAllocator()
	suite.namedWorkerAllocators.Store("mock-allocator", allocator)
}

func (suite *WebsocketFactoryTestSuite) TestFactoryRegistration() {
	f := &factory{}

	// Test factory creation
	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket",
		WorkerAllocatorName: "mock-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr": ":8080",
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	triggerInstance, err := f.Create(suite.logger, "test-websocket", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.NoError(err)
	suite.NotNil(triggerInstance)

	// Verify it's a websocket trigger
	_, ok := triggerInstance.(*websocket_trigger)
	suite.True(ok)
}

func (suite *WebsocketFactoryTestSuite) TestFactoryWithInvalidWorkerAllocator() {
	f := &factory{}

	triggerConfig := &functionconfig.Trigger{
		Kind:                "websocket",
		Name:                "test-websocket",
		WorkerAllocatorName: "non-existent-allocator",
		Attributes: map[string]interface{}{
			"websocket_addr": ":8080",
		},
	}

	runtimeConfig := &runtime.Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Spec: functionconfig.Spec{
					Runtime: "python",
					Handler: "test_handler:handler",
				},
			},
		},
	}

	triggerInstance, err := f.Create(suite.logger, "test-websocket", triggerConfig, runtimeConfig, suite.namedWorkerAllocators, nil)
	suite.Error(err)
	suite.Nil(triggerInstance)
}

func TestWebsocketFactoryTestSuite(t *testing.T) {
	suite.Run(t, new(WebsocketFactoryTestSuite))
}
