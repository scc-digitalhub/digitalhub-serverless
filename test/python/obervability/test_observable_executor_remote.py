import os
import unittest
from unittest.mock import Mock
import sys
from os.path import dirname

# Add the py directory to sys.path to import observable_executor
sys.path.insert(0,dirname(dirname(dirname(dirname(__file__)))))

from pkg.processor.runtime.python.py.observable_executor import (
    initialize_opentelemetry,
    execute_callable,
    OTE_TRACING_CONTENT,
)
from nuclio_sdk import Response, Event, Context


class TestObservableExecutor(unittest.TestCase):

    def test_process_input_basic(self):
        """Test _process_input with basic event."""
        mock_event = Event()
        mock_event.method = 'GET'
        mock_event.url = 'http://example.com/path?query=value'
        mock_event.path = '/path'
        mock_event.headers = {'User-Agent': 'test-agent', 'X-Forwarded-For': '192.168.1.1'}
        mock_event.size = 100
        mock_event.body = b'test body'

        os.environ[OTE_TRACING_CONTENT] = 'true'
        os.environ["OTEL_SERVICE_NAME"] = "test-function3"
        os.environ["OTEL_TRACES_EXPORTER"] = "otlp"
        os.environ["OTEL_METRICS_EXPORTER"] = "otlp"
        os.environ["OTEL_EXPORTER_OTLP_PROTOCOL"] = "grpc"
        os.environ["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://localhost:4317"
        os.environ["OTEL_PROPAGATORS"] = "tracecontext,baggage"
        mock_ctx = Context()
        mock_ctx.logger = Mock()

        initialize_opentelemetry(mock_ctx)

        def function(context, event):
            print("Function executed")
            return "done"

        try:
            execute_callable(function, event=mock_event, context=mock_ctx)
        except Exception as e:
            self.fail(f"execute_callable raised an exception: {e}")

        print("Test complete")
        


if __name__ == '__main__':
    unittest.main()