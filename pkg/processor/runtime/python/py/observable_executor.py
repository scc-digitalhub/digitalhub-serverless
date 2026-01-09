import os
import time

from attr import attrs
from opentelemetry import trace, metrics
from opentelemetry.util import types

import opentelemetry.semconv.attributes.http_attributes as http_attributes
import opentelemetry.semconv.attributes.url_attributes as url_attributes
import opentelemetry.semconv.attributes.server_attributes as server_attributes
import opentelemetry.semconv.attributes.client_attributes as client_attributes
import opentelemetry.semconv.attributes.network_attributes as network_attributes
import opentelemetry.semconv.attributes.user_agent_attributes as user_agent_attributes

from opentelemetry.semconv.metrics.http_metrics import (
    HTTP_SERVER_REQUEST_DURATION,

)
from opentelemetry.semconv._incubating.metrics.http_metrics import (
    create_http_server_active_requests,
)

from opentelemetry.sdk._configuration import _OTelSDKConfigurator
import typing
from urllib.parse import urlparse

from nuclio_sdk import Context, Event, Response

class OpenTelemetryConfigurator(_OTelSDKConfigurator):
    pass

HTTP_REQUEST_BODY_SIZE = "http.request.body.size"
HTTP_REQUEST_SIZE = "http.request.size"
HTTP_REQUEST_BODY_VALUE = "http.request.body.value"
HTTP_RESPONSE_BODY_SIZE = "http.response.body.size"
HTTP_RESPONSE_SIZE = "http.response.size"
HTTP_RESPONSE_BODY_VALUE = "http.response.body.value"

DURATION_ATTRS = [
    http_attributes.HTTP_REQUEST_METHOD, 
    url_attributes.URL_SCHEME,
    http_attributes.HTTP_ROUTE, 
    server_attributes.SERVER_ADDRESS, 
    server_attributes.SERVER_PORT,
    http_attributes.HTTP_RESPONSE_STATUS_CODE
    ]

ACTIVE_REQUEST_COUNTER_ATTRS = [
    http_attributes.HTTP_REQUEST_METHOD, 
    url_attributes.URL_SCHEME,
    server_attributes.SERVER_ADDRESS, 
    server_attributes.SERVER_PORT,
    ]

OTE_TRACING_CONTENT = "OTEL_TRACING_CONTENT"

HTTP_DURATION_HISTOGRAM_BUCKETS = (
    0.005,
    0.01,
    0.025,
    0.05,
    0.075,
    0.1,
    0.25,
    0.5,
    0.75,
    1,
    2.5,
    5,
    7.5,
    10,
)

def is_tracing_enabled() -> bool:
    """
    Check if tracing is enabled via env vars.

    Returns
    -------
    bool
        True if tracing is enabled, False otherwise.
    """
    return True

def is_metrics_enabled() -> bool:
    """
    Check if metrics is enabled via env vars.

    Returns
    -------
    bool
        True if metrics is enabled, False otherwise.
    """
    return True

def is_content_tracing_enabled() -> bool:
    """
    Check if content tracing is enabled via env vars.

    Returns
    -------
    bool
        True if content tracing is enabled, False otherwise.
    """
    content_tracing = os.getenv(OTE_TRACING_CONTENT, "false").lower()
    return content_tracing == "true"

def initialize_opentelemetry(ctx: Context) -> None:
    """
    Initialize OpenTelemetry using env vars.

    Parameters
    ----------
    ctx : Context
        Nuclio context.
    """
    OpenTelemetryConfigurator().configure()
    run = getattr(ctx, "run", None)
    tracer_name = run.id if run else "serverless-function"

    tracer = trace.get_tracer(tracer_name)
    setattr(ctx, "tracer", tracer)
    setattr(ctx, "content_tracing_enabled", is_content_tracing_enabled())

    meter = metrics.get_meter(tracer_name)
    setattr(ctx, "meter", meter)
    # histogram metric http.server.request.duration (required)
    duration_histogram = meter.create_histogram(
                name=HTTP_SERVER_REQUEST_DURATION,
                description="Duration of HTTP server requests.",
                unit="s",
                explicit_bucket_boundaries_advisory=HTTP_DURATION_HISTOGRAM_BUCKETS,
            )
    setattr(ctx, "meter_duration_histogram", duration_histogram)
    # updowncounter metric http.server.active_requests (optional)
    active_requests_counter = create_http_server_active_requests(meter)
    setattr(ctx, "meter_active_requests_counter", active_requests_counter)

    # histogram metric http.server.request.body.size (optional)
    request_size_histogram = meter.create_histogram( 
                name="http.server.request.body.size",
                description="Size of HTTP server request bodies.",
                unit="By",
            )
    setattr(ctx, "meter_request_body_size_histogram", request_size_histogram)

    response_size_histogram = meter.create_histogram( 
                name="http.server.response.body.size",
                description="Size of HTTP server response bodies.",
                unit="By",
            )
    setattr(ctx, "meter_response_body_size_histogram", response_size_histogram)

    ctx.logger.info("OpenTelemetry initialized.")

def execute_callable(function: typing.Callable, **kwargs):
    ctx = kwargs.get("context")
    tracer = getattr(ctx, "tracer")

    attributes: typing.Mapping[str, types.AttributeValue] = {}

    if tracer is None or not is_tracing_enabled():
        return _execute_measured(ctx, attributes, function, **kwargs)
    
    with tracer.start_as_current_span("span-name") as span:
        try:
            if "event" in kwargs:
                _process_input(kwargs["event"], ctx, attributes)
            return  _execute_measured(ctx, attributes, function, **kwargs)
        except Exception as ex:
            current_span = trace.get_current_span()
            current_span.set_status(trace.Status(trace.StatusCode.ERROR))
            current_span.record_exception(ex)
            raise ex
        finally:
            span.set_attributes(attributes)

def _process_input(event: Event, ctx: Context, attributes: typing.Mapping[str, types.AttributeValue]):
    """
    Process input event and set span attributes.
    Parameters
    ----------
    span : trace.Span
        OpenTelemetry span.
    event : Event
        Nuclio event.
    ctx : Context
        Nuclio context.
    attributes : typing.Mapping[str, types.AttributeValue]
        Attributes container to set.
    """
    attributes[http_attributes.HTTP_REQUEST_METHOD] = str(event.method)
    if event.url:
        parsed_url = urlparse(event.url)
        attributes[url_attributes.URL_PATH] = parsed_url.path
        attributes[url_attributes.URL_SCHEME] = parsed_url.scheme
        attributes[server_attributes.SERVER_ADDRESS] = parsed_url.hostname
        if parsed_url.port:
            attributes[server_attributes.SERVER_PORT] = parsed_url.port
        if parsed_url.query:
            attributes[url_attributes.URL_QUERY] = parsed_url.query
        attributes[url_attributes.URL_FULL] = event.url
    else:
        attributes[url_attributes.URL_PATH] = event.path or ""
        attributes[url_attributes.URL_SCHEME] = "http"
        attributes[url_attributes.URL_FULL] = ""

    attributes[http_attributes.HTTP_ROUTE] = event.path or ""
    if event.headers:
        for header_key, header_value in event.headers.items():
            header_attr_key = f"{http_attributes.HTTP_REQUEST_HEADER_TEMPLATE}.{header_key.lower()}"
            attributes[header_attr_key] = header_value
    if event.size is not None:
        attributes[HTTP_REQUEST_SIZE] = event.size
    if event.body:
        if ctx.content_tracing_enabled:
            attributes[HTTP_REQUEST_BODY_VALUE] = event.body
        attributes[HTTP_REQUEST_BODY_SIZE] = len(event.body)
    # Set additional attributes from headers
    if event.headers:
        user_agent = event.headers.get('User-Agent') or event.headers.get('user-agent')
        if user_agent:
            attributes[user_agent_attributes.USER_AGENT_ORIGINAL] = user_agent
        client_addr = event.headers.get('X-Forwarded-For') or event.headers.get('X-Real-IP')
        if client_addr:
            # Take the first IP if comma separated
            addr = client_addr.split(',')[0].strip()
            attributes[client_attributes.CLIENT_ADDRESS] = addr
            # Assuming network.peer is the same for now
            attributes[network_attributes.NETWORK_PEER_ADDRESS] = addr
        # For protocol version, from X-Forwarded-Proto
        proto = event.headers.get('X-Forwarded-Proto')
        if proto:
            attributes[network_attributes.NETWORK_PROTOCOL_VERSION] = proto
        # For port, from X-Forwarded-Port
        port = event.headers.get('X-Forwarded-Port')
        if port:
            try:
                port_int = int(port)
                attributes[network_attributes.NETWORK_PEER_PORT] = port_int
            except ValueError:
                pass


def _process_output(response: any, ctx: Context, attributes: typing.Mapping[str, types.AttributeValue]):
    """
    Process output response and set span attributes.
    Parameters
    ----------
    span : trace.Span
        OpenTelemetry span.
    ctx : Context
        Nuclio context.
    attributes : typing.Mapping[str, types.AttributeValue]
        Attributes container to set.
    response : any
        User function response.
    """
    if isinstance(response, Response): 
        attributes[http_attributes.HTTP_RESPONSE_STATUS_CODE] = response.status_code
        if response.headers:
            for header_key, header_value in response.headers.items():
                header_attr_key = f"{http_attributes.HTTP_RESPONSE_HEADER_TEMPLATE}.{header_key.lower()}"
                attributes[header_attr_key] = header_value
        if response.size is not None:
            attributes[HTTP_RESPONSE_SIZE] = response.size
        if response.body:
            if ctx.content_tracing_enabled:
                attributes[HTTP_RESPONSE_BODY_VALUE] = response.body
            attributes[HTTP_RESPONSE_BODY_SIZE] = len(response.body)
    else:
        attributes[http_attributes.HTTP_RESPONSE_STATUS_CODE] = 200
        if ctx.content_tracing_enabled:
            attributes[HTTP_RESPONSE_BODY_VALUE] = response
        attributes[HTTP_RESPONSE_BODY_SIZE] = len(response)

def _execute_measured(ctx: Context, attributes: typing.Mapping[str, types.AttributeValue], function: typing.Callable, **kwargs):
    """
    Execute function and measure duration.
    Parameters
    ----------
    ctx : Context
        Nuclio context.
    attributes : typing.Mapping[str, types.AttributeValue]
        Attributes container to set.
    function : function
        User function to execute.
    """
    meter = getattr(ctx, "meter")
    duration_histogram = getattr(ctx, "meter_duration_histogram")
    if meter is None or duration_histogram is None or not is_metrics_enabled():
        return function(**kwargs)

    active_request_counter = getattr(ctx, "meter_active_requests_counter")
    # Increase active requests counter
    if active_request_counter:
        active_request_counter.add(1, _filter_attrs(attributes, ACTIVE_REQUEST_COUNTER_ATTRS))

    request_body_size_histogram = getattr(ctx, "meter_request_body_size_histogram")
    response_body_size_histogram = getattr(ctx, "meter_response_body_size_histogram")

    start_time = time.time()
    try:
        res = function(**kwargs)
        _process_output(res, ctx, attributes)        
    finally:
        end_time = time.time()
        duration = end_time - start_time
        # Record duration metric
        duration_histogram.record(duration, _filter_attrs(attributes, DURATION_ATTRS))
        # Decrease active requests counter
        if active_request_counter:
            active_request_counter.add(-1, _filter_attrs(attributes, ACTIVE_REQUEST_COUNTER_ATTRS))
        # Record request/response body size metrics
        if request_body_size_histogram and HTTP_REQUEST_BODY_SIZE in attributes:
            request_body_size_histogram.record(
                attributes[HTTP_REQUEST_BODY_SIZE],
                _filter_attrs(attributes, DURATION_ATTRS),
            )
        if response_body_size_histogram and HTTP_RESPONSE_BODY_SIZE in attributes:
            response_body_size_histogram.record(
                attributes[HTTP_RESPONSE_BODY_SIZE],
                _filter_attrs(attributes, DURATION_ATTRS),
            )

def _filter_attrs(attrs: typing.Mapping[str, types.AttributeValue], names: list[str]) -> dict[str, types.AttributeValue]:
    """
    Filter attributes.
    Parameters
    ----------
    attrs : typing.Mapping[str, types.AttributeValue]
        Attributes container.
    names : list[str]
        Names to filter.
    Returns
    -------
    dict[str, types.AttributeValue]
        Filtered attributes.
    """
    filtered_attrs = {}
    for key, val in attrs.items():
        if key in names:
            filtered_attrs[key] = val
    return filtered_attrs