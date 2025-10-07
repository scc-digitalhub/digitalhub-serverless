/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/
package extproc

import (
	"log"
)

/**
 * PostProcessor pattern:
 * -  modifies response body or leave it unchanged
 * -  in case of error, logs it and leaves body unchanged
**/
type PostProcessor struct {
	AbstractProcessor
}

func (s *PostProcessor) processResponse(ctx *RequestContext, body []byte) ([]byte, error) {
	res, err := s.Handler.HandleEvent(ctx, body)
	if err != nil {
		return body, err
	}
	return res.Body, nil

}

func (s *PostProcessor) GetName() string {
	return "postprocessor"
}

func (s *PostProcessor) ProcessResponseHeaders(ctx *RequestContext, headers AllHeaders) error {
	return ctx.ContinueRequest()
}

func (s *PostProcessor) ProcessResponseBody(ctx *RequestContext, body []byte) error {
	processed, err := s.processResponse(ctx, body)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		ctx.ReplaceBodyChunk(processed)
	}
	return ctx.ContinueRequest()
}
