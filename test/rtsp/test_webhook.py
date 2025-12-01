from flask import Flask, request

app = Flask(__name__)

@app.route("/audio", methods=["POST"])
def receive():
    print("=== GOT POST ===")
    print(request.json)       # print JSON payload
    print("================")
    return "ok", 200

app.run(host="0.0.0.0", port=8080)
