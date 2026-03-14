import json

import requests
from fastapi import FastAPI
from fastapi.responses import StreamingResponse
from pydantic import BaseModel

app = FastAPI()

OLLAMA_URL = "http://localhost:11434/api/generate"
OLLAMA_MODEL = "llama3"


class GenerateRequest(BaseModel):
    prompt: str


def stream_ollama(prompt: str):
    payload = {"model": OLLAMA_MODEL, "prompt": prompt, "stream": True}

    with requests.post(OLLAMA_URL, json=payload, stream=True) as r:
        for line in r.iter_lines():
            if line:
                data = json.loads(line)
                token = data.get("response", "")
                yield token


@app.post("/generate")
def generate(req: GenerateRequest):
    payload = {"model": OLLAMA_MODEL, "prompt": req.prompt, "stream": False}

    return StreamingResponse(stream_ollama(req.prompt), media_type="text/plain")
