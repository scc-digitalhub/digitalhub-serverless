/*
Copyright 2023 The DH Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
