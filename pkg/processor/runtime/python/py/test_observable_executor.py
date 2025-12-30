import os
import unittest
from unittest.mock import Mock, patch, MagicMock
import sys
import tempfile

# Add the py directory to sys.path to import observable_executor
sys.path.insert(0, os.path.dirname(__file__))

from observable_executor import (
    is_tracing_enabled,
    is_metrics_enabled,
    is_content_tracing_enabled,
    initialize_opentelemetry,
    execute_callable,
    _process_input,
    _process_output,
    OTE_TRACING_CONTENT,
)
from nuclio_sdk import Response


class TestObservableExecutor(unittest.TestCase):

    def test_is_tracing_enabled(self):
        """Test is_tracing_enabled function."""
        self.assertTrue(is_tracing_enabled())

    def test_is_metrics_enabled(self):
        """Test is_metrics_enabled function."""
        self.assertTrue(is_metrics_enabled())

    def test_is_content_tracing_enabled_false(self):
        """Test is_content_tracing_enabled when env var is not set or false."""
        with patch.dict(os.environ, {}, clear=True):
            self.assertFalse(is_content_tracing_enabled())

        with patch.dict(os.environ, {OTE_TRACING_CONTENT: 'false'}):
            self.assertFalse(is_content_tracing_enabled())

        with patch.dict(os.environ, {OTE_TRACING_CONTENT: 'False'}):
            self.assertFalse(is_content_tracing_enabled())

    def test_is_content_tracing_enabled_true(self):
        """Test is_content_tracing_enabled when env var is true."""
        with patch.dict(os.environ, {OTE_TRACING_CONTENT: 'true'}):
            self.assertTrue(is_content_tracing_enabled())

        with patch.dict(os.environ, {OTE_TRACING_CONTENT: 'True'}):
            self.assertTrue(is_content_tracing_enabled())

    @patch('observable_executor.OpenTelemetryConfigurator')
    @patch('observable_executor.trace.get_tracer')
    def test_initialize_opentelemetry(self, mock_get_tracer, mock_configurator):
        """Test initialize_opentelemetry function."""
        mock_ctx = Mock()
        mock_ctx.logger = Mock()
        mock_ctx.run = Mock()
        mock_ctx.run.id = 'test-run-id'

        mock_tracer = Mock()
        mock_get_tracer.return_value = mock_tracer

        initialize_opentelemetry(mock_ctx)

        mock_configurator.assert_called_once()
        mock_configurator.return_value.configure.assert_called_once()
        mock_get_tracer.assert_called_once_with('test-run-id')
        self.assertEqual(mock_ctx.tracer, mock_tracer)
        self.assertTrue(hasattr(mock_ctx, 'content_tracing_enabled'))
        mock_ctx.logger.info.assert_called_once_with("OpenTelemetry initialized.")

    @patch('observable_executor.OpenTelemetryConfigurator')
    @patch('observable_executor.trace.get_tracer')
    def test_initialize_opentelemetry_no_run(self, mock_get_tracer, mock_configurator):
        """Test initialize_opentelemetry when ctx.run is None."""
        mock_ctx = Mock()
        mock_ctx.logger = Mock()
        mock_ctx.run = None

        mock_tracer = Mock()
        mock_get_tracer.return_value = mock_tracer

        initialize_opentelemetry(mock_ctx)

        mock_get_tracer.assert_called_once_with('serverless-function')

    @patch('observable_executor.trace.get_current_span')
    @patch('observable_executor._process_output')
    @patch('observable_executor._process_input')
    def test_execute_callable_with_event(self, mock_process_input, mock_process_output, mock_get_current_span):
        """Test execute_callable with event in kwargs."""
        mock_ctx = Mock()
        mock_ctx.tracer = Mock()
        mock_span = Mock()
        mock_ctx.tracer.start_as_current_span.return_value.__enter__ = Mock(return_value=mock_span)
        mock_ctx.tracer.start_as_current_span.return_value.__exit__ = Mock(return_value=None)

        mock_function = Mock(return_value='result')
        mock_event = Mock()

        result = execute_callable(mock_function, event=mock_event, context=mock_ctx)

        self.assertEqual(result, 'result')
        mock_ctx.tracer.start_as_current_span.assert_called_once_with("span-name")
        mock_process_input.assert_called_once_with(mock_span, mock_event, mock_ctx)
        mock_process_output.assert_called_once_with(mock_span, 'result', mock_ctx)

    @patch('observable_executor.trace.get_current_span')
    @patch('observable_executor._process_output')
    @patch('observable_executor._process_input')
    def test_execute_callable_without_event(self, mock_process_input, mock_process_output, mock_get_current_span):
        """Test execute_callable without event in kwargs."""
        mock_ctx = Mock()
        mock_ctx.tracer = Mock()
        mock_span = Mock()
        mock_ctx.tracer.start_as_current_span.return_value.__enter__ = Mock(return_value=mock_span)
        mock_ctx.tracer.start_as_current_span.return_value.__exit__ = Mock(return_value=None)

        mock_function = Mock(return_value='result')

        result = execute_callable(mock_function, context=mock_ctx, arg1='value1')

        self.assertEqual(result, 'result')
        mock_process_input.assert_not_called()
        mock_process_output.assert_called_once_with(mock_span, 'result', mock_ctx)

    @patch('observable_executor.trace.get_current_span')
    @patch('observable_executor._process_output')
    @patch('observable_executor._process_input')
    def test_execute_callable_exception(self, mock_process_input, mock_process_output, mock_get_current_span):
        """Test execute_callable when function raises exception."""
        mock_ctx = Mock()
        mock_ctx.tracer = Mock()
        mock_span = Mock()
        mock_ctx.tracer.start_as_current_span.return_value.__enter__ = Mock(return_value=mock_span)
        mock_ctx.tracer.start_as_current_span.return_value.__exit__ = Mock(return_value=None)

        mock_current_span = Mock()
        mock_get_current_span.return_value = mock_current_span

        mock_function = Mock(side_effect=Exception('test error'))

        # The function catches the exception and records it, does not re-raise
        result = execute_callable(mock_function, context=mock_ctx)

        self.assertIsNone(result)  # Since except doesn't return
        mock_current_span.set_status.assert_called_once()
        mock_current_span.record_exception.assert_called_once()

    @patch('observable_executor.urlparse')
    def test_process_input_basic(self, mock_urlparse):
        """Test _process_input with basic event."""
        mock_span = Mock()
        mock_event = Mock()
        mock_event.method = 'GET'
        mock_event.url = 'http://example.com/path?query=value'
        mock_event.path = '/path'
        mock_event.headers = {'User-Agent': 'test-agent', 'X-Forwarded-For': '192.168.1.1'}
        mock_event.size = 100
        mock_event.body = b'test body'

        mock_ctx = Mock()
        mock_ctx.content_tracing_enabled = True

        mock_parsed = Mock()
        mock_parsed.path = '/path'
        mock_parsed.scheme = 'http'
        mock_parsed.hostname = 'example.com'
        mock_parsed.port = 80
        mock_parsed.query = 'query=value'
        mock_urlparse.return_value = mock_parsed

        _process_input(mock_span, mock_event, mock_ctx)

        # Check attribute calls
        mock_span.set_attribute.assert_any_call('http.request.method', 'GET')
        mock_span.set_attribute.assert_any_call('url.path', '/path')
        mock_span.set_attribute.assert_any_call('url.scheme', 'http')
        mock_span.set_attribute.assert_any_call('server.address', 'example.com')
        mock_span.set_attribute.assert_any_call('server.port', 80)
        mock_span.set_attribute.assert_any_call('url.query', 'query=value')
        mock_span.set_attribute.assert_any_call('url.full', 'http://example.com/path?query=value')
        mock_span.set_attribute.assert_any_call('http.route', '/path')
        mock_span.set_attribute.assert_any_call('http.request.header.user-agent', 'test-agent')
        mock_span.set_attribute.assert_any_call('http.request.size', 100)
        mock_span.set_attribute.assert_any_call('http.request.body.value', b'test body')
        mock_span.set_attribute.assert_any_call('http.request.body.size', 9)
        # Additional attributes
        mock_span.set_attribute.assert_any_call('user_agent.original', 'test-agent')
        mock_span.set_attribute.assert_any_call('client.address', '192.168.1.1')
        mock_span.set_attribute.assert_any_call('network.peer.address', '192.168.1.1')

    def test_process_input_no_url(self):
        """Test _process_input with no url."""
        mock_span = Mock()
        mock_event = Mock()
        mock_event.method = 'POST'
        mock_event.url = None
        mock_event.path = '/test'
        mock_event.headers = None
        mock_event.size = None
        mock_event.body = None

        mock_ctx = Mock()
        mock_ctx.content_tracing_enabled = False

        _process_input(mock_span, mock_event, mock_ctx)

        mock_span.set_attribute.assert_any_call('http.request.method', 'POST')
        mock_span.set_attribute.assert_any_call('url.path', '/test')
        mock_span.set_attribute.assert_any_call('url.scheme', 'http')
        mock_span.set_attribute.assert_any_call('url.full', '')
        mock_span.set_attribute.assert_any_call('http.route', '/test')

    def test_process_output_with_response(self):
        """Test _process_output with Response object."""
        mock_span = Mock()
        mock_response = Mock(spec=Response)
        mock_response.status_code = 200
        mock_response.headers = {'Content-Type': 'application/json'}
        mock_response.size = 50
        mock_response.body = b'{"key": "value"}'

        mock_ctx = Mock()
        mock_ctx.content_tracing_enabled = True

        _process_output(mock_span, mock_response, mock_ctx)

        mock_span.set_attribute.assert_any_call('http.response.status_code', 200)
        mock_span.set_attribute.assert_any_call('http.response.header.content-type', 'application/json')
        mock_span.set_attribute.assert_any_call('http.response.size', 50)
        mock_span.set_attribute.assert_any_call('http.response.body.value', b'{"key": "value"}')
        mock_span.set_attribute.assert_any_call('http.response.body.size', 16)

    def test_process_output_without_response(self):
        """Test _process_output with non-Response object."""
        mock_span = Mock()
        mock_response = 'string response'

        mock_ctx = Mock()

        _process_output(mock_span, mock_response, mock_ctx)

        mock_span.set_attribute.assert_called_once_with('http.response.status_code', 200)


if __name__ == '__main__':
    unittest.main()