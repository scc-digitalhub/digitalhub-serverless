import io
import os
import time
import base64
from flask import Flask, request
from ultralytics import YOLOWorld
from PIL import Image

app = Flask(__name__)

# model = YOLOWorld("yolov8s-world.pt")

@app.route("/video", methods=["POST"])
def receive():
    print("=== GOT POST ===")
    print(f"Content-Type: {request.content_type}")
    
    data = None
    text = None
    
    # Check form fields (CreateFormField writes here)
    if request.form:
        print(f"Form fields in request: {list(request.form.keys())}")
        if 'data' in request.form:
            # Form fields with binary data are encoded - need to get raw bytes
            data = request.form['data'].encode('latin-1') if isinstance(request.form['data'], str) else request.form['data']
            print(f"Got data from form: {len(data)} bytes")
        if 'text' in request.form:
            text = request.form['text']
            print(f"Got text from form: {text}")
    
    # Check file uploads (for comparison)
    if request.files:
        print(f"Files in request: {list(request.files.keys())}")
        if 'data' in request.files:
            data = request.files['data'].read()
            print(f"Got data from files: {len(data)} bytes")
    
    if data is None:
        print("ERROR: No data received")
        return "ERROR: No data", 400
    
    ts = int(time.time() * 1000)
    filename = f"frame_{ts}.jpg"
    path = os.path.join("/Users/gosha/Work/FBK/repos/digitalhub-serverless/test/rtsp_video/frames", filename)

    # Verify JPEG header
    if not data.startswith(b'\xff\xd8'):
        print(f"ERROR: Invalid JPEG header. First bytes: {data[:10].hex()}")
        return "ERROR: Invalid JPEG", 400

    try:
        with open(path, "wb") as f:
            f.write(data)
        print(f"Saved frame to {path} ({len(data)} bytes)")
    except Exception as e:
        print(f"ERROR writing file: {e}")
        return "NOT OK"


    print("================")
    return "ok", 200

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080)
