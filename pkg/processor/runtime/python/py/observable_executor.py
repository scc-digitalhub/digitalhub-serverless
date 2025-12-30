import os
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
import opentelemetry.semconv.attributes.http_attributes as http_attributes
import opentelemetry.semconv.attributes.url_attributes as url_attributes
import opentelemetry.semconv.attributes.server_attributes as server_attributes
import opentelemetry.semconv.attributes.client_attributes as client_attributes
import opentelemetry.semconv.attributes.network_attributes as network_attributes
import opentelemetry.semconv.attributes.user_agent_attributes as user_agent_attributes

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


OTE_TRACING_CONTENT = "OTEL_TRACING_CONTENT"

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
    ctx.logger.info("OpenTelemetry initialized.")

def execute_callable(function: typing.Callable, **kwargs):
    ctx = kwargs.get("context")
    tracer = getattr(ctx, "tracer")
    if tracer is None:
        return function(**kwargs)
    
    with tracer.start_as_current_span("span-name") as span:
        try:
            if "event" in kwargs:
                _process_input(span, kwargs["event"], ctx)
            res = function(**kwargs)
            _process_output(span, res, ctx)
            return res
        except Exception as ex:
            current_span = trace.get_current_span()
            current_span.set_status(trace.Status(trace.StatusCode.ERROR))
            current_span.record_exception(ex)

def _process_input(span: trace.Span, event: Event, ctx: Context):
    """
    Process input event and set span attributes.
    Parameters
    ----------
    span : trace.Span
        OpenTelemetry span.
    event : Event
        Nuclio event.
    """
    span.set_attribute(http_attributes.HTTP_REQUEST_METHOD, event.method)
    if event.url:
        parsed_url = urlparse(event.url)
        span.set_attribute(url_attributes.URL_PATH, parsed_url.path)
        span.set_attribute(url_attributes.URL_SCHEME, parsed_url.scheme)
        span.set_attribute(server_attributes.SERVER_ADDRESS, parsed_url.hostname)
        if parsed_url.port:
            span.set_attribute(server_attributes.SERVER_PORT, parsed_url.port)
        if parsed_url.query:
            span.set_attribute(url_attributes.URL_QUERY, parsed_url.query)
        span.set_attribute(url_attributes.URL_FULL, event.url)
    else:
        span.set_attribute(url_attributes.URL_PATH, event.path or "")
        span.set_attribute(url_attributes.URL_SCHEME, "http")
        span.set_attribute(url_attributes.URL_FULL, "")

    span.set_attribute(http_attributes.HTTP_ROUTE, event.path or "")
    if event.headers:
        for header_key, header_value in event.headers.items():
            header_attr_key = f"{http_attributes.HTTP_REQUEST_HEADER_TEMPLATE}.{header_key.lower()}"
            span.set_attribute(header_attr_key, header_value)
    if event.size is not None:
        span.set_attribute(HTTP_REQUEST_SIZE, event.size)
    if event.body:
        if ctx.content_tracing_enabled:
            span.set_attribute(HTTP_REQUEST_BODY_VALUE, event.body)
        span.set_attribute(HTTP_REQUEST_BODY_SIZE, len(event.body))
    # Set additional attributes from headers
    if event.headers:
        user_agent = event.headers.get('User-Agent') or event.headers.get('user-agent')
        if user_agent:
            span.set_attribute(user_agent_attributes.USER_AGENT_ORIGINAL, user_agent)
        client_addr = event.headers.get('X-Forwarded-For') or event.headers.get('X-Real-IP')
        if client_addr:
            # Take the first IP if comma separated
            addr = client_addr.split(',')[0].strip()
            span.set_attribute(client_attributes.CLIENT_ADDRESS, addr)
            # Assuming network.peer is the same for now
            span.set_attribute(network_attributes.NETWORK_PEER_ADDRESS, addr)
        # For protocol version, from X-Forwarded-Proto
        proto = event.headers.get('X-Forwarded-Proto')
        if proto:
            span.set_attribute(network_attributes.NETWORK_PROTOCOL_VERSION, proto)
        # For port, from X-Forwarded-Port
        port = event.headers.get('X-Forwarded-Port')
        if port:
            try:
                port_int = int(port)
                span.set_attribute(network_attributes.NETWORK_PEER_PORT, port_int)
            except ValueError:
                pass


def _process_output(span, response: any, ctx: Context):
    """
    Process output response and set span attributes.
    Parameters
    ----------
    span : trace.Span
        OpenTelemetry span.
    response : any
        User function response.
    """
    if isinstance(response, Response): 
        span.set_attribute(http_attributes.HTTP_RESPONSE_STATUS_CODE, response.status_code)
        if response.headers:
            for header_key, header_value in response.headers.items():
                header_attr_key = f"{http_attributes.HTTP_RESPONSE_HEADER_TEMPLATE}.{header_key.lower()}"
                span.set_attribute(header_attr_key, header_value)
        if response.size is not None:
            span.set_attribute(HTTP_RESPONSE_SIZE, response.size)
        if response.body:
            if ctx.content_tracing_enabled:
                span.set_attribute(HTTP_RESPONSE_BODY_VALUE, response.body)
            span.set_attribute(HTTP_RESPONSE_BODY_SIZE, len(response.body))
    else:
        span.set_attribute(http_attributes.HTTP_RESPONSE_STATUS_CODE, 200)
        if ctx.content_tracing_enabled:
            span.set_attribute(HTTP_RESPONSE_BODY_VALUE, response)
        span.set_attribute(HTTP_RESPONSE_BODY_SIZE, len(response))
