/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package mjpeg

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/stretchr/testify/require"
)

func TestFactory(t *testing.T) {
	t.Run("FactoryRegistration", func(t *testing.T) {
		// Verify that the factory is registered
		registeredFactory, err := trigger.RegistrySingleton.Get("mjpeg")
		require.NoError(t, err)
		require.NotNil(t, registeredFactory)
	})
}
