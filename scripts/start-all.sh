#!/usr/bin/env bash
# Mac launcher: starts Kokoro TTS + tts-player + agent/TUI.
# Bridge + StS2 game must already be running on the configured Windows host.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# Load per-machine overrides if present. Use `set -a` so sourced assignments
# become environment variables for child processes (agent/TUI/tts-player)
# without needing explicit `export` on every line.
if [[ -f "$REPO_ROOT/.env.local" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$REPO_ROOT/.env.local"
  set +a
fi

: "${SPIRE2MIND_BRIDGE_HOST:=127.0.0.1}"
: "${SPIRE2MIND_BRIDGE_PORT:=8080}"
: "${SPIRE2MIND_BRIDGE_URL:=http://${SPIRE2MIND_BRIDGE_HOST}:${SPIRE2MIND_BRIDGE_PORT}}"

: "${SPIRE2MIND_MODEL_PROVIDER:=api}"
: "${SPIRE2MIND_API_PROVIDER:=openai}"
: "${SPIRE2MIND_API_BASE_URL:=http://127.0.0.1:11434}"
: "${SPIRE2MIND_API_KEY:=}"
: "${SPIRE2MIND_API_DECISION_MODE:=structured}"
: "${SPIRE2MIND_MODEL:=qwen3.5:35b-a3b-coding-nvfp4}"
: "${SPIRE2MIND_MODEL_CONTEXT:=32768}"

: "${KOKORO_PORT:=18081}"
: "${KOKORO_VOICE:=zf_xiaoxiao}"
: "${KOKORO_LANG:=z}"

: "${SPIRE2MIND_TTS_PROVIDER:=kokoro}"
: "${SPIRE2MIND_TTS_FALLBACK_PROVIDER:=kokoro}"
: "${SPIRE2MIND_TTS_BASE_URL:=http://127.0.0.1:${KOKORO_PORT}}"
: "${SPIRE2MIND_TTS_MODEL:=kokoro}"
: "${SPIRE2MIND_TTS_VOICE:=${KOKORO_VOICE}}"
: "${SPIRE2MIND_TTS_FORMAT:=wav}"

export SPIRE2MIND_BRIDGE_URL
export SPIRE2MIND_MODEL_PROVIDER SPIRE2MIND_API_PROVIDER
export SPIRE2MIND_API_BASE_URL SPIRE2MIND_API_KEY
export SPIRE2MIND_API_DECISION_MODE SPIRE2MIND_MODEL SPIRE2MIND_MODEL_CONTEXT
export SPIRE2MIND_TTS_PROVIDER SPIRE2MIND_TTS_FALLBACK_PROVIDER
export SPIRE2MIND_TTS_BASE_URL SPIRE2MIND_TTS_MODEL
export SPIRE2MIND_TTS_VOICE SPIRE2MIND_TTS_FORMAT

KOKORO_VENV="$REPO_ROOT/.tools/kokoro/venv"
KOKORO_PY="$KOKORO_VENV/bin/python"
if [[ ! -x "$KOKORO_PY" ]]; then
  echo "error: Kokoro venv not found at $KOKORO_VENV" >&2
  echo "bootstrap with: /opt/homebrew/bin/python3.12 -m venv $KOKORO_VENV && $KOKORO_VENV/bin/pip install kokoro fastapi uvicorn ordered-set pypinyin cn2an jieba" >&2
  exit 1
fi

SCRATCH_DIR="$REPO_ROOT/scratch/tts"
mkdir -p "$SCRATCH_DIR"
KOKORO_LOG="$SCRATCH_DIR/kokoro.log"
PLAYER_LOG="$SCRATCH_DIR/player.log"

# Clean up any orphaned processes from previous runs before starting new ones
cleanup_orphans() {
  set +e
  local orphan_count
  orphan_count=$(pgrep -f "node.*tts-player/index.mjs" 2>/dev/null | wc -l | tr -d ' ')
  if [[ "$orphan_count" -gt 0 ]]; then
    echo "cleaning up $orphan_count orphaned tts-player process(es)..."
    pkill -f "node.*tts-player/index.mjs" 2>/dev/null || true
    sleep 0.5
  fi

  # Also check for orphaned go run processes
  orphan_count=$(pgrep -f "go run.*cmd/spire2mind" 2>/dev/null | wc -l | tr -d ' ')
  if [[ "$orphan_count" -gt 0 ]]; then
    echo "cleaning up $orphan_count orphaned agent process(es)..."
    pkill -f "go run.*cmd/spire2mind" 2>/dev/null || true
    sleep 0.5
  fi
}
cleanup_orphans

KOKORO_PID=""
PLAYER_PID=""
AGENT_PID=""

# Clean up all spawned processes and any orphaned tts-player instances
cleanup() {
  set +e
  echo "cleaning up..."

  # Kill agent if we started it
  if [[ -n "$AGENT_PID" ]] && kill -0 "$AGENT_PID" 2>/dev/null; then
    kill "$AGENT_PID" 2>/dev/null
    wait "$AGENT_PID" 2>/dev/null
  fi

  # Kill tts-player if we started it
  if [[ -n "$PLAYER_PID" ]] && kill -0 "$PLAYER_PID" 2>/dev/null; then
    kill "$PLAYER_PID" 2>/dev/null
  fi

  # Kill Kokoro if we started it
  if [[ -n "$KOKORO_PID" ]] && kill -0 "$KOKORO_PID" 2>/dev/null; then
    kill "$KOKORO_PID" 2>/dev/null
  fi

  # Also clean up any orphaned tts-player processes from previous runs
  pkill -f "node.*tts-player/index.mjs" 2>/dev/null || true

  echo "cleanup done"
}
trap cleanup EXIT INT TERM

start_kokoro() {
  if curl -fsS -m 2 "http://127.0.0.1:${KOKORO_PORT}/health" >/dev/null 2>&1; then
    echo "kokoro already running on :${KOKORO_PORT}"
    return
  fi
  echo "starting kokoro on :${KOKORO_PORT}..."
  SPIRE2MIND_KOKORO_LANGUAGE_CODE="$KOKORO_LANG" \
  SPIRE2MIND_KOKORO_VOICE="$KOKORO_VOICE" \
    "$KOKORO_PY" -m uvicorn kokoro_server:app \
      --app-dir "$REPO_ROOT/tools/local-tts" \
      --host 127.0.0.1 --port "$KOKORO_PORT" \
      >"$KOKORO_LOG" 2>&1 &
  KOKORO_PID=$!
  for _ in {1..60}; do
    if curl -fsS -m 2 "http://127.0.0.1:${KOKORO_PORT}/health" >/dev/null 2>&1; then
      echo "kokoro ready (pid $KOKORO_PID)"
      return
    fi
    sleep 1
  done
  echo "error: kokoro did not become ready; see $KOKORO_LOG" >&2
  exit 1
}

start_player() {
  echo "starting tts-player..."
  node "$REPO_ROOT/tools/tts-player/index.mjs" >>"$PLAYER_LOG" 2>&1 &
  PLAYER_PID=$!
  echo "tts-player pid $PLAYER_PID"
}

check_bridge() {
  echo "checking bridge at $SPIRE2MIND_BRIDGE_URL..."
  if curl -fsS -m 5 "${SPIRE2MIND_BRIDGE_URL}/health" >/dev/null 2>&1; then
    echo "bridge reachable"
  else
    echo "warning: bridge not reachable at $SPIRE2MIND_BRIDGE_URL (StS2 not running or LAN issue)" >&2
  fi
}

start_kokoro
start_player
check_bridge

echo "launching agent/TUI (bridge=$SPIRE2MIND_BRIDGE_URL)"
go run ./cmd/spire2mind play "$@" &
AGENT_PID=$!
wait $AGENT_PID
