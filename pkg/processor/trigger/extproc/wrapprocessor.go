package extproc

import (
	"log"
)

type ImmediateResponse struct {
	Status  int32
	Headers map[string]HeaderValue
	Body    []byte
}

/**
 * WrapProcessor pattern:
 * -  modifies request body or leave it unchanged and decide whether return response or continue
 * -  in case of error, logs it and leaves body unchanged
**/
type WrapProcessor struct {
	AbstractProcessor
}

func (s *WrapProcessor) wrapRequest(ctx *RequestContext, body []byte) ([]byte, *ImmediateResponse, error) {
	res, err := s.Handler.HandleEvent(ctx, body)

	// in case of error, return original body and the error
	if err != nil {
		return nil, nil, err
	}

	// if response is not nil and status > 0, return it as immediate response
	if res != nil {
		if res.Status > 0 {
			ir := &ImmediateResponse{
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
func (s *WrapProcessor) wrapResponse(ctx *RequestContext, body []byte) ([]byte, error) {
	res, err := s.Handler.HandleEvent(ctx, body)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

func (s *WrapProcessor) GetName() string {
	return "wrapprocessor"
}

func (s *WrapProcessor) ProcessRequestHeaders(ctx *RequestContext, headers AllHeaders) error {
	// TODO: not needed if ProcessRequestBody always called
	// _, err, res := wrapRequest(ctx, nil)
	// if err != nil {
	// 	log.Printf("Error: %v", err)
	// } else if res != nil {
	// 	return ctx.CancelRequest(res.Status, res.Headers, res.Body)
	// }
	return ctx.ContinueRequest()
}

func (s *WrapProcessor) ProcessRequestBody(ctx *RequestContext, body []byte) error {
	_, res, err := s.wrapRequest(ctx, nil)
	if err != nil {
		log.Printf("Error: %v", err)
	} else if res != nil {
		return ctx.CancelRequest(res.Status, res.Headers, res.Body)
	} else {
		ctx.ReplaceBodyChunk(body)
	}
	return ctx.ContinueRequest()
}

func (s *WrapProcessor) ProcessResponseHeaders(ctx *RequestContext, headers AllHeaders) error {
	// TODO: not needed if ProcessResponseBody always called
	// _, err := wrapResponse(ctx, nil)
	// if err != nil {
	// 	log.Printf("Error: %v", err)
	// }
	return ctx.ContinueRequest()
}

func (s *WrapProcessor) ProcessResponseBody(ctx *RequestContext, body []byte) error {
	processed, err := s.wrapResponse(ctx, body)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		ctx.ReplaceBodyChunk(processed)
	}
	return ctx.ContinueRequest()
}
