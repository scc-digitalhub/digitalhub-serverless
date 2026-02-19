#!/usr/bin/env python3
#
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0
#
"""
WebSocket Test Server for receiving frames from MJPEG trigger.

This server receives binary frames via WebSocket and can display them or save them.
Run this in a separate terminal before starting the mjpeg-websocket test.
"""
import asyncio
import websockets
import sys

async def handle_client(websocket):
    """Handle incoming WebSocket connections and messages."""
    print(f"Client connected from {websocket.remote_address}")
    
    frame_count = 0
    try:
        async for message in websocket:
            frame_count += 1
            if isinstance(message, bytes):
                print(f"Received frame {frame_count}: {len(message)} bytes")
                
                # Optional: Save frames to disk
                # with open(f"frame_{frame_count}.jpg", "wb") as f:
                #     f.write(message)
            else:
                print(f"Received text message: {message}")
    except websockets.exceptions.ConnectionClosed:
        print(f"Client disconnected. Total frames received: {frame_count}")
    except Exception as e:
        print(f"Error: {e}")
        import traceback
        traceback.print_exc()

async def main():
    """Start the WebSocket server."""
    port = 9090
    print(f"Starting WebSocket server on ws://localhost:{port}/frames")
    print("Waiting for connections...")
    
    async with websockets.serve(handle_client, "localhost", port):
        await asyncio.Future()  # run forever

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nServer stopped")
        sys.exit(0)
