/*
SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler

SPDX-License-Identifier: Apache-2.0
*/
package extproc

import (
	"errors"
	"io"
	"log"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
)

const (
	ProcessingPhaseHeader string = "processing-phase"
)

// Primary interface for supported request processing that SDK users must
// implement, passing a complying type to `GenericExtProcServer` or `Serve`.
//
// TODO: Passing through health check calls would help support better reasoning
// about dependencies for external processing (e.g., DB or kafka availability)
type RequestProcessor interface {
	GetName() string
	GetOptions() *ProcessingOptions

	ProcessRequestHeaders(ctx *RequestContext, headers AllHeaders) error
	ProcessRequestTrailers(ctx *RequestContext, trailers AllHeaders) error
	ProcessResponseHeaders(ctx *RequestContext, headers AllHeaders) error
	ProcessResponseTrailers(ctx *RequestContext, trailers AllHeaders) error

	ProcessResponseBody(ctx *RequestContext, body []byte) error
	ProcessRequestBody(ctx *RequestContext, body []byte) error
}

type EventResponse struct {
	Status  int
	Headers map[string]string
	Body    []byte
}

type EventHandler interface {
	HandleEvent(ctx *RequestContext, body []byte) (*EventResponse, error)
}

type AbstractProcessor struct {
	opts *ProcessingOptions

	RequestProcessor

	Handler EventHandler
}

func (s *AbstractProcessor) Init(opts *ProcessingOptions, nonFlagArgs []string, handler EventHandler) error {
	s.opts = opts
	s.Handler = handler
	return nil
}

func (s *AbstractProcessor) Finish() {}

func (s *AbstractProcessor) GetOptions() *ProcessingOptions {
	return s.opts
}

func (s *AbstractProcessor) ProcessResponseTrailers(ctx *RequestContext, trailers AllHeaders) error {
	return ctx.ContinueRequest()
}
func (s *AbstractProcessor) ProcessRequestTrailers(ctx *RequestContext, trailers AllHeaders) error {
	return ctx.ContinueRequest()
}
func (s *AbstractProcessor) ProcessRequestHeaders(ctx *RequestContext, headers AllHeaders) error {
	return ctx.ContinueRequest()
}
func (s *AbstractProcessor) ProcessResponseHeaders(ctx *RequestContext, headers AllHeaders) error {
	return ctx.ContinueRequest()
}
func (s *AbstractProcessor) ProcessResponseBody(ctx *RequestContext, body []byte) error {
	return ctx.ContinueRequest()
}
func (s *AbstractProcessor) ProcessRequestBody(ctx *RequestContext, body []byte) error {
	return ctx.ContinueRequest()
}

// Definition for DRY body handling
type BodyHandler func(*RequestContext, []byte) error

// Generic type for an external processor to which we can attach a gRPC bidi stream
// implementation.
type GenericExtProcServer struct {
	name      string
	processor RequestProcessor
	options   *ProcessingOptions
}

// Implementation of the bidi stream `Process` in an external processor. Given the
// type data `processor` and `options`, this intends to manage construction and
// updating of a `RequestContext` and calls to the `processor`'s phase-specific
// methods.
func (s *GenericExtProcServer) Process(srv extprocv3.ExternalProcessor_ProcessServer) error {
	if s.processor == nil {
		log.Fatalf("cannot process request stream without `processor` interface")
	}

	if s.options == nil {
		s.options = NewDefaultOptions()
	}

	if s.options.LogStream {
		log.Printf("Starting request stream in \"%s\"", s.name)
	}

	rc := &RequestContext{
		extProcOptions: s.options,
	}
	ctx := srv.Context()

	for {
		select {
		case <-ctx.Done():
			if s.options.LogStream {
				log.Printf("Request stream terminated in \"%s\"", s.name)
			}
			return ctx.Err()

		default:
		}

		req, err := srv.Recv()
		if err == io.EOF {
			if s.options.LogStream {
				log.Printf("Request stream terminated in \"%s\"", s.name)
			}
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Unknown, "cannot receive stream request: %v", err)
		}

		// clear response in the context if defined, this is not
		// carried across request phases because each one has an
		// idiosyncratic response. rc gets "initialized" during
		// RequestHeaders phase processing.
		_ = rc.ResetPhase()

		resp, err := s.processPhase(req, s.processor, rc)
		if err != nil {
			log.Printf("Phase processing error %v\n", err)
		} else if resp == nil {
			log.Printf("Phase processing did not define a response\n")
		} else {
			if s.options.LogPhases {
				log.Printf("Sending ProcessingResponse: %v \n", resp)
			}
			if err := srv.Send(resp); err != nil {
				log.Printf("Send error %v", err)
			}
		}
	} // end for over stream messages
}

// Internal per-phase processing logic, with a defined `RequestContext` and `RequestProcessor`
func (s *GenericExtProcServer) processPhase(procReq *extprocv3.ProcessingRequest, processor RequestProcessor, rc *RequestContext) (*extprocv3.ProcessingResponse, error) {
	if rc == nil {
		log.Printf("WARNING: RequestContext is undefined (nil)\n")
	}

	var (
		ps  time.Time
		err error
	)

	phase := REQUEST_PHASE_UNDETERMINED

	switch req := procReq.Request.(type) {
	case *extprocv3.ProcessingRequest_RequestHeaders:
		phase = REQUEST_PHASE_REQUEST_HEADERS
		if s.options.LogPhases {
			log.Printf("Processing Request Headers: %v \n", req)
		}
		h := req.RequestHeaders

		ah, _ := NewAllHeadersFromEnvoyHeaderMap(h.Headers)
		ah.Headers[ProcessingPhaseHeader] = strconv.Itoa(phase)

		// initialize request context (requires _not_ skipping request headers)
		_ = initReqCtx(rc, &ah)
		rc.EndOfStream = h.EndOfStream

		// set content-type, content-encoding, and/or transfer-encoding as available
		rc.bodybuffer = NewEncodedBodyFromHeaders(rc.AllHeaders)

		ps = time.Now()
		err = processor.ProcessRequestHeaders(rc, *rc.AllHeaders)
		rc.Duration += time.Since(ps)

	case *extprocv3.ProcessingRequest_RequestBody:
		phase = REQUEST_PHASE_REQUEST_BODY
		if s.options.LogPhases {
			log.Printf("Processing Request Body: %v \n", req)
		}
		b := req.RequestBody
		rc.EndOfStream = b.EndOfStream
		rc.AllHeaders.Headers[ProcessingPhaseHeader] = strconv.Itoa(phase)

		ps = time.Now()
		err = rc.handleBodyChunk(processor.ProcessRequestBody, s.options, b.Body)
		rc.Duration += time.Since(ps)

	case *extprocv3.ProcessingRequest_RequestTrailers:
		phase = REQUEST_PHASE_REQUEST_TRAILERS
		if s.options.LogPhases {
			log.Printf("Processing Request Trailers: %v \n", req)
		}
		ts := req.RequestTrailers

		trailers, _ := NewAllHeadersFromEnvoyHeaderMap(ts.Trailers)
		rc.AllHeaders.Headers[ProcessingPhaseHeader] = strconv.Itoa(phase)

		ps = time.Now()
		err = processor.ProcessRequestTrailers(rc, trailers)
		rc.Duration += time.Since(ps)

	case *extprocv3.ProcessingRequest_ResponseHeaders:
		phase = REQUEST_PHASE_RESPONSE_HEADERS
		if s.options.LogPhases {
			log.Printf("Processing Response Headers: %v \n", req)
		}
		hs := req.ResponseHeaders
		rc.EndOfStream = hs.EndOfStream
		rc.AllHeaders.Headers[ProcessingPhaseHeader] = strconv.Itoa(phase)

		// _response_ headers

		headers, _ := NewAllHeadersFromEnvoyHeaderMap(hs.Headers)

		// set status (ignoring error if found, 0 default)
		_ = rc.parseStatusFromResponseHeaders(headers)

		// remove "envoy" headers from (copied) headers, so clients don't need to parse
		headers.DropHeadersNamedStartingWith(":")

		rc.AllHeaders = &headers

		// set content-type, content-encoding, and/or transfer-encoding as available
		rc.bodybuffer = NewEncodedBodyFromHeaders(&headers)

		ps = time.Now()
		err = processor.ProcessResponseHeaders(rc, headers)
		rc.Duration += time.Since(ps)

		if s.options.UpdateExtProcHeader {
			rc.AppendHeader("x-extproc-names", HeaderValue{RawValue: []byte(s.name)})
		}
		if rc.EndOfStream && s.options.UpdateDurationHeader {
			rc.AppendHeader("x-extproc-duration-ns", HeaderValue{RawValue: []byte(strconv.FormatInt(rc.Duration.Nanoseconds(), 10))})
		}

	case *extprocv3.ProcessingRequest_ResponseBody:
		phase = REQUEST_PHASE_RESPONSE_BODY
		if s.options.LogPhases {
			log.Printf("Processing Response Body: %v \n", req)
		}
		b := req.ResponseBody
		rc.EndOfStream = b.EndOfStream
		rc.AllHeaders.Headers[ProcessingPhaseHeader] = strconv.Itoa(phase)

		ps = time.Now()
		err = rc.handleBodyChunk(processor.ProcessResponseBody, s.options, b.Body)
		rc.Duration += time.Since(ps)

		if rc.EndOfStream && s.options.UpdateDurationHeader {
			rc.AppendHeader("x-extproc-duration-ns", HeaderValue{RawValue: []byte(strconv.FormatInt(rc.Duration.Nanoseconds(), 10))})
		}

	case *extprocv3.ProcessingRequest_ResponseTrailers:
		phase = REQUEST_PHASE_RESPONSE_TRAILERS
		if s.options.LogPhases {
			log.Printf("Processing Response Trailers: %v \n", req)
		}
		ts := req.ResponseTrailers

		trailers, _ := NewAllHeadersFromEnvoyHeaderMap(ts.Trailers)

		ps = time.Now()
		err = processor.ProcessResponseTrailers(rc, trailers)
		rc.Duration += time.Since(ps)

	default:
		if s.options.LogPhases {
			log.Printf("Unknown Request type: %v\n", req)
		}
		err = errors.New("unknown request type")
	}
	if err != nil {
		return nil, err
	}

	return rc.GetResponse(phase)
}
