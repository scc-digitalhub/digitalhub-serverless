/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package tvm

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
)

type factory struct{}

func (f *factory) Create(parentLogger logger.Logger,
	configuration *runtime.Configuration) (runtime.Runtime, error) {
	return NewRuntime(parentLogger.GetChild("tvm"), configuration)
}

// registered via the blank-import in cmd/processor/app/processor.go.
func init() {
	runtime.RegistrySingleton.Register("tvm", &factory{})
}
