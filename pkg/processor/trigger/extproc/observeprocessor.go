/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/
package extproc

import (
	"log"
)

/**
 * ObserveProcessor pattern:
 * -  handles request response without modifying them
**/
type ObserveProcessor struct {
	AbstractProcessor
}

func (s *ObserveProcessor) observeRequest(ctx *RequestContext, body []byte) error {
	_, err := s.Handler.HandleEvent(ctx, body)
	return err

}
func (s *ObserveProcessor) observeResponse(ctx *RequestContext, body []byte) error {
	_, err := s.Handler.HandleEvent(ctx, body)
	return err

}

func (s *ObserveProcessor) GetName() string {
	return "observeprocessor"
}

func (s *ObserveProcessor) ProcessRequestHeaders(ctx *RequestContext, headers AllHeaders) error {
	if !ctx.HasBody() {
		err := s.observeRequest(ctx, nil)
		if err != nil {
			log.Printf("Error: %v", err)
		}
	}
	return ctx.ContinueRequest()
}

func (s *ObserveProcessor) ProcessRequestBody(ctx *RequestContext, body []byte) error {
	err := s.observeRequest(ctx, body)
	if err != nil {
		log.Printf("Error: %v", err)
	}
	return ctx.ContinueRequest()
}

func (s *ObserveProcessor) ProcessResponseHeaders(ctx *RequestContext, headers AllHeaders) error {
	return ctx.ContinueRequest()
}

func (s *ObserveProcessor) ProcessResponseBody(ctx *RequestContext, body []byte) error {
	err := s.observeResponse(ctx, body)
	if err != nil {
		log.Printf("Error: %v", err)
	}
	return ctx.ContinueRequest()
}
