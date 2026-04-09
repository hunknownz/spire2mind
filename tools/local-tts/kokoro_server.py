import io
import os
import wave
from functools import lru_cache

import numpy as np
from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse, Response
from kokoro import KPipeline
from pydantic import BaseModel, Field


APP_TITLE = "Spire2Mind Kokoro Server"
DEFAULT_LANGUAGE_CODE = os.getenv("SPIRE2MIND_KOKORO_LANGUAGE_CODE", "z").strip() or "z"
DEFAULT_VOICE = os.getenv("SPIRE2MIND_KOKORO_VOICE", "zf_xiaoxiao").strip() or "zf_xiaoxiao"
DEFAULT_SPEED = float(os.getenv("SPIRE2MIND_KOKORO_SPEED", "1.0"))
DEFAULT_SPLIT_PATTERN = os.getenv("SPIRE2MIND_KOKORO_SPLIT_PATTERN", r"\n+")


class SpeechRequest(BaseModel):
    model: str | None = None
    voice: str | None = None
    input: str = Field(min_length=1)
    response_format: str | None = "wav"
    speed: float | None = None


app = FastAPI(title=APP_TITLE)


@lru_cache(maxsize=1)
def get_pipeline() -> KPipeline:
    return KPipeline(lang_code=DEFAULT_LANGUAGE_CODE)


def to_wav_bytes(audio: np.ndarray, sample_rate: int) -> bytes:
    clipped = np.clip(audio, -1.0, 1.0)
    pcm = (clipped * 32767.0).astype(np.int16)
    buffer = io.BytesIO()
    with wave.open(buffer, "wb") as wav_file:
        wav_file.setnchannels(1)
        wav_file.setsampwidth(2)
        wav_file.setframerate(sample_rate)
        wav_file.writeframes(pcm.tobytes())
    return buffer.getvalue()


def resolve_voice(voice: str | None) -> str:
    requested = (voice or "").strip()
    if not requested:
        return DEFAULT_VOICE
    normalized = requested.lower()
    aliases = {
        "female": DEFAULT_VOICE,
        "cute": "zf_xiaoxiao",
        "bright": "zf_xiaoyi",
        "soft": "zf_xiaobei",
    }
    return aliases.get(normalized, requested)


def synthesize_numpy(text: str, voice: str, speed: float) -> tuple[np.ndarray, int]:
    pipeline = get_pipeline()
    audio_chunks: list[np.ndarray] = []
    sample_rate = 24000

    generator = pipeline(
        text,
        voice=voice,
        speed=speed,
        split_pattern=DEFAULT_SPLIT_PATTERN,
    )
    for _, _, audio in generator:
        if audio is None:
            continue
        chunk = np.asarray(audio, dtype=np.float32).reshape(-1)
        if chunk.size == 0:
            continue
        audio_chunks.append(chunk)

    if not audio_chunks:
        raise RuntimeError("kokoro returned no audio")

    return np.concatenate(audio_chunks), sample_rate


@app.get("/health")
def health() -> JSONResponse:
    pipeline = get_pipeline()
    return JSONResponse(
        {
            "status": "ok",
            "engine": "kokoro",
            "language_code": DEFAULT_LANGUAGE_CODE,
            "default_voice": DEFAULT_VOICE,
            "sample_rate": 24000,
            "pipeline": type(pipeline).__name__,
        }
    )


def synthesize(req: SpeechRequest) -> Response:
    response_format = (req.response_format or "wav").strip().lower()
    if response_format != "wav":
        raise HTTPException(status_code=400, detail="Only wav output is supported by the local Kokoro server.")

    text = req.input.strip()
    if not text:
        raise HTTPException(status_code=400, detail="input must be non-empty")

    speed = req.speed if req.speed is not None else DEFAULT_SPEED
    voice = resolve_voice(req.voice)

    try:
        audio, sample_rate = synthesize_numpy(text, voice=voice, speed=speed)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"synthesis failed: {exc}") from exc

    wav_bytes = to_wav_bytes(audio, sample_rate)
    return Response(content=wav_bytes, media_type="audio/wav")


@app.post("/audio/speech")
def audio_speech(req: SpeechRequest) -> Response:
    return synthesize(req)


@app.post("/v1/audio/speech")
def v1_audio_speech(req: SpeechRequest) -> Response:
    return synthesize(req)
