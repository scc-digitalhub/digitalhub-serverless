#!/usr/bin/env python3
#
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0
#
"""
Webhook Test Server for receiving frame data from MJPEG trigger.

This server receives POST requests with frame metadata and analysis results.
Run this in a separate terminal before starting the mjpeg-webhook test.
"""
from http.server import HTTPServer, BaseHTTPRequestHandler
import json
import sys

class WebhookHandler(BaseHTTPRequestHandler):
    """Handle incoming webhook POST requests."""
    
    request_count = 0
    
    def do_POST(self):
        """Handle POST requests."""
        WebhookHandler.request_count += 1
        
        # Check the path
        if self.path != '/webhook':
            self.send_response(404)
            self.end_headers()
            return
        
        # Check API key header
        api_key = self.headers.get('X-API-Key')
        if api_key != 'test-api-key-12345':
            print(f"WARNING: Invalid API key: {api_key}")
        
        # Read the request body
        content_length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(content_length)
        
        # Parse JSON
        try:
            data = json.loads(body.decode('utf-8'))
            
            
            print(f"\n{'='*60}")
            print(f"Request #{WebhookHandler.request_count} received")
            print(f"{'='*60}")

            print(f"transcription: {data.get('transcription')}")
            print(f"size: {data.get('size')} bytes")
            
            if data.get('thumbnail'):
                print(f"Thumbnail: {len(data.get('thumbnail'))} chars (base64)")
            
            # Send success response
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            
            response = {
                "status": "success",
                "message": "Frame data received",
                "request_id": WebhookHandler.request_count
            }
            self.wfile.write(json.dumps(response).encode('utf-8'))
            
        except json.JSONDecodeError as e:
            print(f"ERROR: Failed to parse JSON: {e}")
            print(f"Body: {body[:200]}")
            
            self.send_response(400)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            
            error_response = {
                "status": "error",
                "message": "Invalid JSON"
            }
            self.wfile.write(json.dumps(error_response).encode('utf-8'))
    
    def log_message(self, format, *args):
        """Override to customize logging."""
        # Suppress default logging
        pass

def main():
    """Start the webhook server."""
    port = 8888
    server_address = ('', port)
    
    print(f"Starting Webhook server on http://localhost:{port}/webhook")
    print(f"API Key: test-api-key-12345")
    print("Waiting for requests...")
    print("Press Ctrl+C to stop\n")
    
    httpd = HTTPServer(server_address, WebhookHandler)
    
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        print(f"\n\nServer stopped. Total requests received: {WebhookHandler.request_count}")
        sys.exit(0)

if __name__ == "__main__":
    main()
