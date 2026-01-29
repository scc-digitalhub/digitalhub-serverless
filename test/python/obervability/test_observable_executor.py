import os
import unittest
from unittest.mock import Mock, patch
import sys
from os.path import dirname

# Add the py directory to sys.path to import observable_executor
sys.path.insert(0, dirname(dirname(dirname(dirname(__file__)))))

from pkg.processor.runtime.python.py.observable_executor import (
    is_tracing_enabled,
    is_metrics_enabled,
    get_profiles,
    initialize_opentelemetry,
    execute_callable,
    _process_request,
    _process_response,
    _execute_measured,
    _filter_attrs,
    OTE_TRACING_CONTENT,
    HTTPProfileProcessor,
    ProfileProcessor,
)
from nuclio_sdk import Response


class TestObservableExecutor(unittest.TestCase):

    def test_is_tracing_enabled(self):
        """Test is_tracing_enabled function."""
        self.assertTrue(is_tracing_enabled())

    def test_is_metrics_enabled(self):
        """Test is_metrics_enabled function."""
        self.assertTrue(is_metrics_enabled())

    def test_get_profiles_default(self):
        """Test get_profiles with default env."""
        with patch.dict(os.environ, {}, clear=True):
            self.assertEqual(get_profiles(), ["http"])

    def test_get_profiles_custom(self):
        """Test get_profiles with custom env."""
        with patch.dict(os.environ, {"OTEL_ENABLED_PROFILES": "http,custom"}):
            self.assertEqual(get_profiles(), ["http", "custom"])

    def test_get_profiles_empty(self):
        """Test get_profiles with empty env."""
        with patch.dict(os.environ, {"OTEL_ENABLED_PROFILES": ""}):
            self.assertEqual(get_profiles(), ["http"])

    @patch('pkg.processor.runtime.python.py.observable_executor.HTTPProfileProcessor')
    @patch('pkg.processor.runtime.python.py.observable_executor.metrics.get_meter')
    @patch('pkg.processor.runtime.python.py.observable_executor.OpenTelemetryConfigurator')
    @patch('pkg.processor.runtime.python.py.observable_executor.trace.get_tracer')
    def test_initialize_opentelemetry(self, mock_get_tracer, mock_configurator, mock_get_meter, mock_http_processor):
        """Test initialize_opentelemetry function."""
        mock_ctx = Mock()
        mock_ctx.logger = Mock()
        mock_ctx.run = Mock()
        mock_ctx.run.id = 'test-run-id'

        mock_tracer = Mock()
        mock_get_tracer.return_value = mock_tracer
        mock_meter = Mock()
        mock_get_meter.return_value = mock_meter

        mock_processor_instance = Mock()
        mock_http_processor.return_value = mock_processor_instance

        initialize_opentelemetry(mock_ctx)

        mock_configurator.assert_called_once()
        mock_configurator.return_value.configure.assert_called_once()
        mock_get_tracer.assert_called_once_with('test-run-id')
        mock_get_meter.assert_called_once_with('test-run-id')
        self.assertEqual(mock_ctx.tracer, mock_tracer)
        self.assertEqual(mock_ctx.meter, mock_meter)
        self.assertTrue(hasattr(mock_ctx, 'profile_processors'))
        self.assertEqual(len(mock_ctx.profile_processors), 1)
        mock_http_processor.assert_called_once()
        mock_processor_instance.init_profile.assert_called_once_with(mock_ctx)
        mock_ctx.logger.info.assert_called_once_with("OpenTelemetry initialized.")

    @patch('pkg.processor.runtime.python.py.observable_executor.HTTPProfileProcessor')
    @patch('pkg.processor.runtime.python.py.observable_executor.metrics.get_meter')
    @patch('pkg.processor.runtime.python.py.observable_executor.OpenTelemetryConfigurator')
    @patch('pkg.processor.runtime.python.py.observable_executor.trace.get_tracer')
    def test_initialize_opentelemetry_no_run(self, mock_get_tracer, mock_configurator, mock_get_meter, mock_http_processor):
        """Test initialize_opentelemetry when ctx.run is None."""
        mock_ctx = Mock()
        mock_ctx.logger = Mock()
        mock_ctx.run = None

        mock_tracer = Mock()
        mock_get_tracer.return_value = mock_tracer
        mock_meter = Mock()
        mock_get_meter.return_value = mock_meter

        mock_processor_instance = Mock()
        mock_http_processor.return_value = mock_processor_instance

        initialize_opentelemetry(mock_ctx)

        mock_get_tracer.assert_called_once_with('serverless-function')
        mock_get_meter.assert_called_once_with('serverless-function')

    @patch('pkg.processor.runtime.python.py.observable_executor._execute_measured')
    @patch('pkg.processor.runtime.python.py.observable_executor._process_request')
    def test_execute_callable_with_event(self, mock_process_request, mock_execute_measured):
        """Test execute_callable with event in kwargs."""
        mock_execute_measured.return_value = None
        mock_ctx = Mock()
        mock_ctx.tracer = Mock()
        mock_span = Mock()
        mock_ctx.tracer.start_as_current_span.return_value.__enter__ = Mock(return_value=mock_span)
        mock_ctx.tracer.start_as_current_span.return_value.__exit__ = Mock(return_value=None)

        mock_function = Mock(return_value='result')
        mock_event = Mock()

        result = execute_callable(mock_function, event=mock_event, context=mock_ctx)

        self.assertIsNone(result)
        mock_ctx.tracer.start_as_current_span.assert_called_once_with("handler")
        # _process_request is called with event, ctx, attributes
        mock_process_request.assert_called_once()
        args = mock_process_request.call_args[0]
        self.assertEqual(args[0], mock_event)
        self.assertEqual(args[1], mock_ctx)
        self.assertIsInstance(args[2], dict)
        mock_execute_measured.assert_called_once_with(mock_ctx, args[2], mock_function, event=mock_event, context=mock_ctx)
        mock_span.set_attributes.assert_called_once_with(args[2])

    @patch('pkg.processor.runtime.python.py.observable_executor._execute_measured')
    @patch('pkg.processor.runtime.python.py.observable_executor._process_request')
    def test_execute_callable_without_event(self, mock_process_request, mock_execute_measured):
        """Test execute_callable without event in kwargs."""
        mock_execute_measured.return_value = None
        mock_ctx = Mock()
        mock_ctx.tracer = Mock()
        mock_span = Mock()
        mock_ctx.tracer.start_as_current_span.return_value.__enter__ = Mock(return_value=mock_span)
        mock_ctx.tracer.start_as_current_span.return_value.__exit__ = Mock(return_value=None)

        mock_function = Mock(return_value='result')

        result = execute_callable(mock_function, context=mock_ctx, arg1='value1')

        self.assertIsNone(result)
        mock_process_request.assert_not_called()
        mock_execute_measured.assert_called_once()
        args = mock_execute_measured.call_args[0]
        self.assertEqual(args[0], mock_ctx)
        self.assertIsInstance(args[1], dict)
        self.assertEqual(args[2], mock_function)
        mock_span.set_attributes.assert_called_once_with(args[1])

    @patch('pkg.processor.runtime.python.py.observable_executor.trace.get_current_span')
    @patch('pkg.processor.runtime.python.py.observable_executor._execute_measured')
    @patch('pkg.processor.runtime.python.py.observable_executor._process_request')
    def test_execute_callable_exception(self, mock_process_request, mock_execute_measured, mock_get_current_span):
        """Test execute_callable when function raises exception."""
        mock_ctx = Mock()
        mock_ctx.tracer = Mock()
        mock_span = Mock()
        mock_ctx.tracer.start_as_current_span.return_value.__enter__ = Mock(return_value=mock_span)
        mock_ctx.tracer.start_as_current_span.return_value.__exit__ = Mock(return_value=None)

        mock_current_span = Mock()
        mock_get_current_span.return_value = mock_current_span

        mock_function = Mock(side_effect=Exception('test error'))
        mock_execute_measured.side_effect = Exception('test error')

        with self.assertRaises(Exception):
            execute_callable(mock_function, context=mock_ctx)

        mock_current_span.set_status.assert_called_once()
        mock_current_span.record_exception.assert_called_once()

    def test_process_request_basic(self):
        """Test _process_request with basic event."""
        attributes = {}
        mock_event = Mock()
        mock_ctx = Mock()
        mock_processor = Mock()
        mock_ctx.profile_processors = [mock_processor]

        _process_request(mock_event, mock_ctx, attributes)

        mock_processor.process_request.assert_called_once_with(mock_event, mock_ctx, attributes)

    def test_process_request_no_url(self):
        """Test _process_request with no url."""
        attributes = {}
        mock_event = Mock()
        mock_ctx = Mock()
        mock_processor = Mock()
        mock_ctx.profile_processors = [mock_processor]

        _process_request(mock_event, mock_ctx, attributes)

        mock_processor.process_request.assert_called_once_with(mock_event, mock_ctx, attributes)

    def test_process_response_with_response(self):
        """Test _process_response with Response object."""
        attributes = {}
        mock_response = Mock()
        start_time = 1.0
        end_time = 2.0
        mock_ctx = Mock()
        mock_processor = Mock()
        mock_ctx.profile_processors = [mock_processor]

        _process_response(mock_response, start_time, end_time, mock_ctx, attributes)

        mock_processor.process_response.assert_called_once_with(mock_response, start_time, end_time, mock_ctx, attributes)

    def test_process_response_without_response(self):
        """Test _process_response with non-Response object."""
        attributes = {}
        mock_response = 'string response'
        start_time = 1.0
        end_time = 2.0
        mock_ctx = Mock()
        mock_processor = Mock()
        mock_ctx.profile_processors = [mock_processor]

        _process_response(mock_response, start_time, end_time, mock_ctx, attributes)

        mock_processor.process_response.assert_called_once_with(mock_response, start_time, end_time, mock_ctx, attributes)

    @patch('pkg.processor.runtime.python.py.observable_executor._process_response')
    @patch('pkg.processor.runtime.python.py.observable_executor.time')
    def test_execute_measured_with_meter(self, mock_time, mock_process_response):
        """Test _execute_measured with meter."""
        mock_time.time.side_effect = [1.0, 2.0]
        mock_ctx = Mock()
        mock_meter = Mock()
        mock_ctx.meter = mock_meter
        mock_histogram = Mock()
        mock_ctx.meter_duration_histogram = mock_histogram
        mock_counter = Mock()
        mock_ctx.meter_active_requests_counter = mock_counter
        mock_request_histogram = Mock()
        mock_ctx.meter_request_body_size_histogram = mock_request_histogram
        mock_response_histogram = Mock()
        mock_ctx.meter_response_body_size_histogram = mock_response_histogram

        attributes = {'http.request.body.size': 100, 'http.response.body.size': 200}
        mock_function = Mock(return_value='result')

        result = _execute_measured(mock_ctx, attributes, mock_function, arg='value')

        self.assertIsNone(result)  # _execute_measured doesn't return the result
        mock_function.assert_called_once_with(arg='value')
        # Metrics recording is done in _process_response
        mock_process_response.assert_called_once()

    def test_execute_measured_no_meter(self):
        """Test _execute_measured without meter."""
        mock_ctx = Mock()
        mock_ctx.meter = None
        attributes = {}
        mock_function = Mock(return_value='result')

        result = _execute_measured(mock_ctx, attributes, mock_function)

        self.assertEqual(result, 'result')
        mock_function.assert_called_once_with()

    def test_filter_attrs(self):
        """Test _filter_attrs function."""
        attrs = {'a': 1, 'b': 2, 'c': 3}
        names = ['a', 'c']
        result = _filter_attrs(attrs, names)
        self.assertEqual(result, {'a': 1, 'c': 3})

    @patch('pkg.processor.runtime.python.py.observable_executor.create_http_server_active_requests')
    @patch('pkg.processor.runtime.python.py.observable_executor.metrics')
    def test_http_profile_processor_init_profile(self, mock_metrics, mock_create_counter):
        """Test HTTPProfileProcessor.init_profile."""
        mock_ctx = Mock()
        mock_meter = Mock()
        mock_ctx.meter = mock_meter
        mock_histogram = Mock()
        mock_meter.create_histogram.return_value = mock_histogram
        mock_counter = Mock()
        mock_create_counter.return_value = mock_counter

        processor = HTTPProfileProcessor()
        processor.init_profile(mock_ctx)

        self.assertFalse(mock_ctx.content_tracing_enabled)
        self.assertEqual(mock_ctx.meter_duration_histogram, mock_histogram)
        self.assertEqual(mock_ctx.meter_active_requests_counter, mock_counter)
        self.assertEqual(mock_ctx.meter_request_body_size_histogram, mock_histogram)
        self.assertEqual(mock_ctx.meter_response_body_size_histogram, mock_histogram)

    @patch('pkg.processor.runtime.python.py.observable_executor.urlparse')
    def test_http_profile_processor_process_request(self, mock_urlparse):
        """Test HTTPProfileProcessor.process_request."""
        mock_parsed = Mock()
        mock_parsed.path = '/path'
        mock_parsed.scheme = 'http'
        mock_parsed.hostname = 'example.com'
        mock_parsed.port = 80
        mock_parsed.query = 'query=value'
        mock_urlparse.return_value = mock_parsed

        processor = HTTPProfileProcessor()
        mock_event = Mock()
        mock_event.method = 'GET'
        mock_event.url = 'http://example.com/path?query=value'
        mock_event.path = '/path'
        mock_event.headers = {'User-Agent': 'agent', 'X-Forwarded-For': '1.2.3.4'}
        mock_event.size = 100
        mock_event.body = b'body'

        mock_ctx = Mock()
        mock_ctx.content_tracing_enabled = True
        mock_counter = Mock()
        mock_ctx.meter_active_requests_counter = mock_counter

        attributes = {}

        processor.process_request(mock_event, mock_ctx, attributes)

        self.assertEqual(attributes['http.request.method'], 'GET')
        self.assertEqual(attributes['url.full'], 'http://example.com/path?query=value')
        self.assertEqual(attributes['user_agent.original'], 'agent')
        self.assertEqual(attributes['client.address'], '1.2.3.4')
        # Check counter add with filtered attrs
        expected_filtered = {'http.request.method': 'GET', 'url.scheme': 'http', 'server.address': 'example.com', 'server.port': 80}
        mock_counter.add.assert_called_once_with(1, expected_filtered)

    def test_http_profile_processor_process_response(self):
        """Test HTTPProfileProcessor.process_response."""
        processor = HTTPProfileProcessor()
        mock_response = Mock(spec=Response)
        mock_response.status_code = 200
        mock_response.headers = {'Content-Type': 'json'}
        mock_response.size = 50
        mock_response.body = b'body'

        mock_ctx = Mock()
        mock_ctx.content_tracing_enabled = True
        mock_histogram = Mock()
        mock_ctx.meter_duration_histogram = mock_histogram
        mock_counter = Mock()
        mock_ctx.meter_active_requests_counter = mock_counter
        mock_request_hist = Mock()
        mock_ctx.meter_request_body_size_histogram = mock_request_hist
        mock_response_hist = Mock()
        mock_ctx.meter_response_body_size_histogram = mock_response_hist

        attributes = {'http.request.body.size': 100, 'http.response.body.size': 50}

        processor.process_response(mock_response, 1.0, 2.0, mock_ctx, attributes)

        self.assertEqual(attributes['http.response.status_code'], 200)
        mock_counter.add.assert_called_once_with(-1, {})
        mock_histogram.record.assert_called_once_with(1.0, {'http.response.status_code': 200})
        mock_request_hist.record.assert_called_once_with(100, {'http.response.status_code': 200})
        mock_response_hist.record.assert_called_once_with(4, {'http.response.status_code': 200})

    def test_http_profile_processor_is_content_tracing_enabled(self):
        """Test HTTPProfileProcessor.is_content_tracing_enabled."""
        processor = HTTPProfileProcessor()
        with patch.dict(os.environ, {OTE_TRACING_CONTENT: 'true'}):
            self.assertTrue(processor.is_content_tracing_enabled())
        with patch.dict(os.environ, {OTE_TRACING_CONTENT: 'false'}):
            self.assertFalse(processor.is_content_tracing_enabled())


if __name__ == '__main__':
    unittest.main()