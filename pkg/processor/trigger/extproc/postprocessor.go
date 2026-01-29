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
 * -  if response with Status > 0 is returned, it is sent as immediate response
 * -  in case of error, logs it and leaves body unchanged
**/
type PostProcessor struct {
	AbstractProcessor
}

func (s *PostProcessor) processResponse(ctx *RequestContext, body []byte) ([]byte, *EventResponse, error) {
	res, err := s.Handler.HandleEvent(ctx, body)
	// in case of error, return original body and the error
	if err != nil {
		return body, nil, err
	}
	// if response is not nil and status > 0, return it as immediate response
	if res != nil {
		if res.Status > 0 {
			ir := &EventResponse{
				Status:  int32(res.Status),
				Headers: make(map[string]HeaderValue),
				Body:    res.Body,
			}
			return nil, ir, nil
		}
		// otherwise, return modified body
		return res.Body, nil, nil
	}

	// otherwise, return original body
	return body, nil, nil
}

func (s *PostProcessor) GetName() string {
	return "postprocessor"
}

func (s *PostProcessor) ProcessResponseHeaders(ctx *RequestContext, headers AllHeaders) error {
	return ctx.ContinueRequest()
}

func (s *PostProcessor) ProcessResponseBody(ctx *RequestContext, body []byte) error {
	newBody, res, err := s.processResponse(ctx, body)
	if err != nil {
		log.Printf("Error: %v", err)
	} else if res != nil {
		return ctx.CancelRequest(res.Status, res.Headers, res.Body)
	} else if newBody != nil {
		ctx.ReplaceBodyChunk(newBody)
	}
	return ctx.ContinueRequest()
}
