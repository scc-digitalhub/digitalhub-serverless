/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/scc-digitalhub/digitalhub-serverless/cmd/processor/app"

	"github.com/nuclio/errors"

	_ "github.com/nuclio/nuclio/pkg/processor/webadmin/resource"
)

func run() error {
	configPath := flag.String("config", "/etc/nuclio/config/processor/processor.yaml", "Path of configuration file")
	platformConfigPath := flag.String("platform-config", "/etc/nuclio/config/platform/platform.yaml", "Path of platform configuration file")
	listRuntimes := flag.Bool("list-runtimes", false, "Show runtimes and exit")
	flag.Parse()

	if *listRuntimes {
		runtimeNames := runtime.RegistrySingleton.GetKinds()
		sort.Strings(runtimeNames)
		for _, name := range runtimeNames {
			fmt.Println(name)
		}
		return nil
	}

	processor, err := app.NewProcessor(*configPath, *platformConfigPath)
	if err != nil {
		return err
	}

	return processor.Start()
}

func main() {
	if err := run(); err != nil {
		errors.PrintErrorStack(os.Stderr, err, 5)

		os.Exit(1)
	}
}
