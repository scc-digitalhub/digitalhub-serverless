/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/

package openinference

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"google.golang.org/grpc"
)

type openInference struct {
	trigger.AbstractTrigger
	configuration *Configuration

	// Server instances
	restServer *http.Server
	grpcServer *grpc.Server

	// Context and synchronization
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newTrigger(
	logger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration,
	restartTriggerChan chan trigger.Trigger,
) (trigger.Trigger, error) {

	abstract, err := trigger.NewAbstractTrigger(
		logger,
		workerAllocator,
		&configuration.Configuration,
		"sync",
		"openinference",
		configuration.Name,
		restartTriggerChan,
	)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract trigger")
	}

	ctx, cancel := context.WithCancel(context.Background())

	newTrigger := &openInference{
		AbstractTrigger: abstract,
		configuration:   configuration,
		ctx:             ctx,
		cancel:          cancel,
	}
	newTrigger.Trigger = newTrigger

	logger.InfoWith("OpenInference trigger created",
		"modelName", configuration.ModelName,
		"modelVersion", configuration.ModelVersion,
		"enableREST", configuration.EnableREST,
		"enableGRPC", configuration.EnableGRPC,
		"restPort", configuration.RESTPort,
		"grpcPort", configuration.GRPCPort)

	return newTrigger, nil
}

func (oi *openInference) Start(checkpoint functionconfig.Checkpoint) error {
	oi.Logger.InfoWith("Starting OpenInference trigger",
		"modelName", oi.configuration.ModelName,
		"modelVersion", oi.configuration.ModelVersion)

	// Start REST server if enabled
	if oi.configuration.EnableREST {
		oi.wg.Add(1)
		go func() {
			defer oi.wg.Done()
			if err := oi.startRESTServer(); err != nil {
				oi.Logger.ErrorWith("REST server failed", "error", err)
			}
		}()
	}

	// Start gRPC server if enabled
	if oi.configuration.EnableGRPC {
		oi.wg.Add(1)
		go func() {
			defer oi.wg.Done()
			if err := oi.startGRPCServer(); err != nil {
				oi.Logger.ErrorWith("gRPC server failed", "error", err)
			}
		}()
	}

	oi.Logger.InfoWith("OpenInference trigger started successfully")
	return nil
}

func (oi *openInference) Stop(force bool) (functionconfig.Checkpoint, error) {
	oi.Logger.InfoWith("Stopping OpenInference trigger")

	// Cancel context to signal shutdown
	oi.cancel()

	// Stop REST server
	if oi.restServer != nil {
		if err := oi.restServer.Shutdown(context.Background()); err != nil {
			oi.Logger.WarnWith("Error shutting down REST server", "error", err)
		}
	}

	// Stop gRPC server
	if oi.grpcServer != nil {
		oi.grpcServer.GracefulStop()
	}

	// Wait for goroutines to finish
	oi.wg.Wait()

	oi.Logger.InfoWith("OpenInference trigger stopped")
	return nil, nil
}

func (oi *openInference) GetConfig() map[string]interface{} {
	return common.StructureToMap(oi.configuration)
}

func (oi *openInference) startRESTServer() error {
	addr := fmt.Sprintf(":%d", oi.configuration.RESTPort)

	mux := http.NewServeMux()
	oi.registerRESTHandlers(mux)

	oi.restServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	oi.Logger.InfoWith("Starting REST server", "address", addr)

	if err := oi.restServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return errors.Wrap(err, "REST server failed")
	}

	return nil
}

func (oi *openInference) startGRPCServer() error {
	addr := fmt.Sprintf(":%d", oi.configuration.GRPCPort)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrap(err, "Failed to create gRPC listener")
	}

	oi.grpcServer = grpc.NewServer()
	oi.registerGRPCHandlers(oi.grpcServer)

	oi.Logger.InfoWith("Starting gRPC server", "address", addr)

	if err := oi.grpcServer.Serve(listener); err != nil {
		return errors.Wrap(err, "gRPC server failed")
	}

	return nil
}
