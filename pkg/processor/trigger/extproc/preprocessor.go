/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/
package extproc

import (
	"log"
)

/**
 * PreProcessor pattern:
 * -  modifies request body or leave it unchanged
 * -  in case of error, logs it and leaves body unchanged
**/
type PreProcessor struct {
	AbstractProcessor
}

func (s *PreProcessor) processRequest(ctx *RequestContext, body []byte) ([]byte, error) {
	res, err := s.Handler.HandleEvent(ctx, body)
	if err != nil {
		return body, err
	}
	return res.Body, nil

}

func (s *PreProcessor) GetName() string {
	return "preprocessor"
}

func (s *PreProcessor) ProcessRequestHeaders(ctx *RequestContext, headers AllHeaders) error {
	if !ctx.HasBody() {
		_, err := s.processRequest(ctx, nil)
		if err != nil {
			log.Printf("Error: %v", err)
		}
	}

	return ctx.ContinueRequest()
}

func (s *PreProcessor) ProcessRequestBody(ctx *RequestContext, body []byte) error {
	processed, err := s.processRequest(ctx, body)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		ctx.ReplaceBodyChunk(processed)
	}
	return ctx.ContinueRequest()
}
