package job

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockControlMessageBroker struct {
	controlcommunication.AbstractControlMessageBroker
}

// func (m *mockControlMessageBroker) PublishControlMessage(message *controlcommunication.ControlMessage) error {
// 	return nil
// }

// func (m *mockControlMessageBroker) Subscribe(kind controlcommunication.ControlMessageKind, channel chan *controlcommunication.ControlMessage) error {
// 	return nil
// }

// func (m *mockControlMessageBroker) Unsubscribe(topic string) error {
// 	return nil
// }

// func (acmb *mockControlMessageBroker) WriteControlMessage(message *controlcommunication.ControlMessage) error {
// 	return nil
// }

// func (acmb *mockControlMessageBroker) ReadControlMessage(reader *bufio.Reader) (*controlcommunication.ControlMessage, error) {
// 	return nil, nil
// }

//	func (acmb *mockControlMessageBroker) SendToConsumers(message *controlcommunication.ControlMessage) error {
//		return nil
//	}
func TestJobTrigger(t *testing.T) {
	tests := []struct {
		name          string
		configuration *Configuration
		expectedError bool
	}{
		{
			name:          "valid configuration",
			configuration: &Configuration{},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock logger
			mockLogger := &mockLogger{}

			// Create worker allocator
			workerAllocator, err := worker.WorkerFactorySingleton.CreateFixedPoolWorkerAllocator(mockLogger,
				tt.configuration.NumWorkers,
				&runtime.Configuration{
					ControlMessageBroker: mockControlMessageBroker{
						&controlcommunication.NewAbstractControlMessageBroker(),
					},
				})
			require.NoError(t, err)

			// Create restart trigger channel
			restartTriggerChan := make(chan trigger.Trigger, 1)

			// Create trigger
			triggerInstance, err := newTrigger(mockLogger,
				workerAllocator,
				tt.configuration,
				restartTriggerChan)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, triggerInstance)

				// Test start
				err = triggerInstance.Start(nil)
				require.NoError(t, err)

				// Give some time for the event to be processed
				time.Sleep(100 * time.Millisecond)

				// Test stop
				checkpoint, err := triggerInstance.Stop(false)
				require.NoError(t, err)
				assert.Nil(t, checkpoint)

				// Test config
				config := triggerInstance.GetConfig()
				assert.NotNil(t, config)
				assert.Equal(t, tt.configuration.Name, config["name"])
			}
		})
	}
}
