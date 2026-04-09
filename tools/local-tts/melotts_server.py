import io
import json
import os
import wave
from functools import lru_cache

import numpy as np
from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse, Response
from pydantic import BaseModel, Field

from melo.api import TTS


APP_TITLE = "Spire2Mind MeloTTS Server"
DEFAULT_LANGUAGE = os.getenv("SPIRE2MIND_MELOTTS_LANGUAGE", "ZH").strip().upper() or "ZH"
DEFAULT_DEVICE = os.getenv("SPIRE2MIND_MELOTTS_DEVICE", "cpu").strip() or "cpu"
DEFAULT_SPEAKER = os.getenv("SPIRE2MIND_MELOTTS_SPEAKER", "ZH").strip() or "ZH"
DEFAULT_SPEED = float(os.getenv("SPIRE2MIND_MELOTTS_SPEED", "1.0"))


class SpeechRequest(BaseModel):
    model: str | None = None
    voice: str | None = None
    input: str = Field(min_length=1)
    response_format: str | None = "wav"
    speed: float | None = None


app = FastAPI(title=APP_TITLE)


@lru_cache(maxsize=1)
def get_model() -> TTS:
    return TTS(language=DEFAULT_LANGUAGE, device=DEFAULT_DEVICE)


def get_speaker_ids() -> dict[str, int]:
    return get_model().hps.data.spk2id


def resolve_speaker(voice: str | None) -> int:
    speaker_ids = get_speaker_ids()
    requested = (voice or "").strip()
    normalized = requested.upper()

    if normalized in speaker_ids:
        return speaker_ids[normalized]

    aliases = {
        "FEMALE": DEFAULT_SPEAKER,
        "DEFAULT": DEFAULT_SPEAKER,
        "ZH": "ZH",
        "ZH-FEMALE": "ZH",
        "CN": "ZH",
        "CHINESE": "ZH",
    }
    alias = aliases.get(normalized, DEFAULT_SPEAKER).upper()
    if alias in speaker_ids:
        return speaker_ids[alias]

    if DEFAULT_SPEAKER.upper() in speaker_ids:
        return speaker_ids[DEFAULT_SPEAKER.upper()]

    # Fall back to the first available speaker if the expected Chinese speaker is missing.
    first_key = next(iter(speaker_ids.keys()))
    return speaker_ids[first_key]


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


@app.get("/health")
def health() -> JSONResponse:
    model = get_model()
    return JSONResponse(
        {
            "status": "ok",
            "engine": "melotts",
            "language": DEFAULT_LANGUAGE,
            "device": DEFAULT_DEVICE,
            "speaker": DEFAULT_SPEAKER,
            "sample_rate": model.hps.data.sampling_rate,
            "speakers": sorted(get_speaker_ids().keys()),
        }
    )


def synthesize(req: SpeechRequest) -> Response:
    response_format = (req.response_format or "wav").strip().lower()
    if response_format != "wav":
        raise HTTPException(status_code=400, detail="Only wav output is supported by the local MeloTTS server.")

    text = req.input.strip()
    if not text:
        raise HTTPException(status_code=400, detail="input must be non-empty")

    speed = req.speed if req.speed is not None else DEFAULT_SPEED
    speaker_id = resolve_speaker(req.voice)
    model = get_model()

    try:
        audio = model.tts_to_file(
            text,
            speaker_id=speaker_id,
            output_path=None,
            speed=speed,
            quiet=True,
        )
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"synthesis failed: {exc}") from exc

    wav_bytes = to_wav_bytes(audio, model.hps.data.sampling_rate)
    return Response(content=wav_bytes, media_type="audio/wav")


@app.post("/audio/speech")
def audio_speech(req: SpeechRequest) -> Response:
    return synthesize(req)


@app.post("/v1/audio/speech")
def v1_audio_speech(req: SpeechRequest) -> Response:
    return synthesize(req)
