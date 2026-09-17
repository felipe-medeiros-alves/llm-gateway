#!/usr/bin/env python3
import json
import os
import time

from flask import Flask, request, Response

app = Flask(__name__)


def sse_chunks(text: str):
    for word in text.split():
        chunk = {"choices": [{"delta": {"content": word + " "}, "index": 0}]}
        yield f"data: {json.dumps(chunk)}\n\n"
        time.sleep(0.01)
    yield "data: [DONE]\n\n"


@app.post("/v1/chat/completions")
def openai_chat():
    body = request.get_json(force=True)
    msg = "mock openai response"
    if body.get("stream"):
        return Response(sse_chunks(msg), mimetype="text/event-stream")
    return {
        "id": "cmpl-mock",
        "object": "chat.completion",
        "choices": [
            {
                "index": 0,
                "message": {"role": "assistant", "content": msg},
                "finish_reason": "stop",
            }
        ],
    }


@app.post("/anthropic/v1/messages")
def anthropic_chat():
    body = request.get_json(force=True)
    msg = "mock anthropic response"
    if body.get("stream"):

        def gen():
            for word in msg.split():
                ev = {
                    "type": "content_block_delta",
                    "delta": {"type": "text_delta", "text": word + " "},
                }
                yield f"event: content_block_delta\ndata: {json.dumps(ev)}\n\n"
            yield "event: message_stop\ndata: {}\n\n"

        return Response(gen(), mimetype="text/event-stream")
    return {
        "id": "msg-mock",
        "type": "message",
        "role": "assistant",
        "content": [{"type": "text", "text": msg}],
    }


@app.post("/vllm/v1/chat/completions")
def vllm_chat():
    return openai_chat()


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=int(os.getenv("PORT", "8089")))
