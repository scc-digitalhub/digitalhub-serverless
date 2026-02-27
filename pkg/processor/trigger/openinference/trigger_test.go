/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package openinference

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfiguration(t *testing.T) {
	t.Run("TensorDefSerDe", func(t *testing.T) {
		tensor := TensorDef{
			Name:     "input",
			DataType: "FP32",
			Shape:    []int64{1, 224, 224, 3},
		}

		assert.Equal(t, "input", tensor.Name)
		assert.Equal(t, "FP32", tensor.DataType)
		assert.Len(t, tensor.Shape, 4)
	})
}
