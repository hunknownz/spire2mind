import fs from "node:fs";
import path from "node:path";
import os from "node:os";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const args = new Set(process.argv.slice(2));
const repoRoot =
  process.env.SPIRE2MIND_REPO_ROOT ||
  path.resolve(__dirname, "..", "..");
const ttsRoot =
  process.env.SPIRE2MIND_TTS_ROOT || path.join(repoRoot, "scratch", "tts");
const latestPath = path.join(ttsRoot, "latest.json");
const audioDir = path.join(ttsRoot, "audio");
const logPath = path.join(ttsRoot, "player.log");
const playScript = path.join(repoRoot, "scripts", "tts-play.ps1");
const powershellExe =
  process.env.ComSpec?.toLowerCase().includes("cmd.exe")
    ? "powershell.exe"
    : process.env.SPIRE2MIND_POWERSHELL || "powershell.exe";

const config = {
  provider: (process.env.SPIRE2MIND_TTS_PROVIDER || "windows-sapi").trim().toLowerCase(),
  fallbackProvider: (process.env.SPIRE2MIND_TTS_FALLBACK_PROVIDER || "windows-sapi").trim().toLowerCase(),
  baseUrl: (process.env.SPIRE2MIND_TTS_BASE_URL || "").trim(),
  apiKey: (process.env.SPIRE2MIND_TTS_API_KEY || "").trim(),
  model: (process.env.SPIRE2MIND_TTS_MODEL || "").trim(),
  voice: (process.env.SPIRE2MIND_TTS_VOICE || "").trim(),
  speechSpeed: clampFloat(process.env.SPIRE2MIND_TTS_SPEED, 1.0, 0.7, 1.35),
  responseFormat: (process.env.SPIRE2MIND_TTS_FORMAT || "wav").trim().toLowerCase(),
  pollMs: clampInt(process.env.SPIRE2MIND_TTS_POLL_MS, 900, 200, 5000),
  speakRate: clampInt(process.env.SPIRE2MIND_TTS_SPEAK_RATE, 0, -10, 10),
  dryRun: args.has("--dry-run"),
};

const state = {
  active: null,
  currentIntent: null,
  pendingIntent: null,
  lastSignature: "",
};

await fs.promises.mkdir(ttsRoot, { recursive: true });
await fs.promises.mkdir(audioDir, { recursive: true });

if (args.has("--self-test")) {
  await runSelfTest();
  process.exit(0);
}

log("info", "tts-player started", {
  provider: config.provider,
  fallbackProvider: config.fallbackProvider,
  baseUrl: config.baseUrl || "-",
  model: resolveSpeechModel() || "-",
  voice: config.voice || "-",
  speechSpeed: config.speechSpeed,
  responseFormat: config.responseFormat,
  pollMs: config.pollMs,
  dryRun: config.dryRun,
});

setInterval(async () => {
  try {
    await tick();
  } catch (error) {
    log("error", "tick failed", { error: error?.message || String(error) });
  }
}, config.pollMs);

try {
  await tick();
} catch (error) {
  log("error", "initial tick failed", { error: error?.message || String(error) });
}

process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);

async function tick() {
  if (!fs.existsSync(latestPath)) {
    return;
  }

  const raw = stripBOM(await fs.promises.readFile(latestPath, "utf8"));
  const beat = JSON.parse(raw);
  const segments = normalizeSegments(beat);
  const trigger = String(beat.trigger || "beat").trim() || "beat";
  if (segments.length === 0) {
    return;
  }

  const signature = `${trigger}|${segments.join("||")}`;
  if (signature === state.lastSignature) {
    return;
  }
  state.lastSignature = signature;

  enqueue({
    id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    trigger,
    mood: String(beat.mood || "").trim(),
    commentary: String(beat.commentary || "").trim(),
    segments,
    segmentIndex: 0,
    priority: triggerPriority(trigger),
    createdAt: Date.now(),
  });
}

function enqueue(intent) {
  if (!state.active) {
    void startIntent(intent);
    return;
  }

  if (shouldInterrupt(intent, state.currentIntent)) {
    log("info", "interrupting active speech", {
      from: state.currentIntent?.trigger || "-",
      to: intent.trigger,
    });
    state.pendingIntent = intent;
    stopActive();
    return;
  }

  state.pendingIntent = intent;
  log("info", "queued speech", {
    trigger: intent.trigger,
    priority: intent.priority,
  });
}

function shouldInterrupt(nextIntent, currentIntent) {
  if (!currentIntent) {
    return true;
  }
  if (nextIntent.priority > currentIntent.priority) {
    return true;
  }
  return nextIntent.priority === currentIntent.priority && nextIntent.trigger !== currentIntent.trigger;
}

async function startIntent(intent) {
  state.currentIntent = intent;
  log("info", "start speech", {
    trigger: intent.trigger,
    mood: intent.mood || "-",
    segmentIndex: intent.segmentIndex,
    provider: config.provider,
  });

  try {
    let child = null;
    if (config.dryRun) {
      child = spawn(
        powershellExe,
        ["-NoProfile", "-Command", "Start-Sleep -Milliseconds 200"],
        { stdio: "ignore", windowsHide: true }
      );
    } else {
      child = await spawnSpeechPlayback(intent);
    }

    state.active = child;
    await waitForChild(child);
  } catch (error) {
    log("error", "speech intent failed", {
      trigger: intent.trigger,
      error: error?.message || String(error),
    });
  } finally {
    state.active = null;
    if (state.currentIntent && hasMoreSegments(state.currentIntent) && !state.pendingIntent) {
      state.currentIntent.segmentIndex += 1;
      void startIntent(state.currentIntent);
      return;
    }

    state.currentIntent = null;
    if (state.pendingIntent) {
      const next = state.pendingIntent;
      state.pendingIntent = null;
      void startIntent(next);
    }
  }
}

async function spawnSpeechPlayback(intent) {
  if (shouldUseAudioSpeechProvider()) {
    try {
      const wavPath = await synthesizeToAudio(intent);
      return spawnPowerShell([
        "-ExecutionPolicy",
        "Bypass",
        "-File",
        playScript,
        "-Mode",
        "wav",
        "-WavPath",
        wavPath,
      ]);
    } catch (error) {
      log("warn", "network tts failed, falling back", {
        provider: config.provider,
        fallbackProvider: config.fallbackProvider,
        error: error?.message || String(error),
      });
    }
  }

  return spawnPowerShell([
    "-ExecutionPolicy",
    "Bypass",
    "-File",
    playScript,
    "-Mode",
    "speak",
    "-Text",
    currentSegment(intent),
    "-VoiceName",
    config.voice,
    "-Rate",
    String(config.speakRate),
  ]);
}

async function synthesizeToAudio(intent) {
  const model = resolveSpeechModel();
  if (!model) {
    throw new Error("speech model is not configured");
  }

  const response = await fetch(normalizeSpeechUrl(config.baseUrl), {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(config.apiKey ? { Authorization: `Bearer ${config.apiKey}` } : {}),
    },
    body: JSON.stringify({
      model,
      voice: config.voice || defaultVoiceForProvider(config.provider),
      input: currentSegment(intent),
      speed: config.speechSpeed,
      response_format: config.responseFormat,
    }),
  });

  if (!response.ok) {
    throw new Error(`speech provider returned ${response.status}`);
  }

  const ext = config.responseFormat === "wav" ? "wav" : config.responseFormat;
  const outPath = path.join(audioDir, `${Date.now()}-${sanitizeFileSlug(intent.trigger)}.${ext}`);
  const buffer = Buffer.from(await response.arrayBuffer());
  await fs.promises.writeFile(outPath, buffer);
  return outPath;
}

function shouldUseAudioSpeechProvider() {
  if (!config.baseUrl) {
    return false;
  }
  return ["openai-compatible", "kokoro", "melotts"].includes(config.provider);
}

function resolveSpeechModel() {
  if (config.model) {
    return config.model;
  }
  switch (config.provider) {
    case "kokoro":
      return "kokoro";
    case "melotts":
      return "melotts";
    default:
      return "";
  }
}

function defaultVoiceForProvider(provider) {
  switch (provider) {
    case "kokoro":
    case "melotts":
      return "female";
    default:
      return "female";
  }
}

function normalizeSpeechUrl(baseUrl) {
  const trimmed = baseUrl.replace(/\/+$/, "");
  if (trimmed.endsWith("/audio/speech")) {
    return trimmed;
  }
  return `${trimmed}/audio/speech`;
}

function normalizeSegments(beat) {
  const raw = Array.isArray(beat?.tts_segments) ? beat.tts_segments : [];
  const segments = raw
    .map((item) => String(item || "").trim())
    .filter(Boolean)
    .slice(0, 4);
  if (segments.length > 0) {
    return segments;
  }

  const fallback = String(beat?.tts_text || beat?.commentary || "").trim();
  return fallback ? [fallback] : [];
}

function currentSegment(intent) {
  if (!intent || !Array.isArray(intent.segments) || intent.segments.length === 0) {
    return "";
  }
  const index = Math.max(0, Math.min(intent.segmentIndex || 0, intent.segments.length - 1));
  return intent.segments[index];
}

function hasMoreSegments(intent) {
  return !!intent && Array.isArray(intent.segments) && intent.segmentIndex < intent.segments.length - 1;
}

function spawnPowerShell(argsList) {
  return spawn(powershellExe, ["-NoProfile", ...argsList], {
    stdio: "ignore",
    windowsHide: true,
  });
}

function stopActive() {
  if (!state.active || state.active.killed) {
    return;
  }
  try {
    state.active.kill("SIGTERM");
  } catch {
    // ignore
  }
}

function waitForChild(child) {
  return new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", (code, signal) => {
      if (code === 0 || signal === "SIGTERM" || signal === "SIGINT") {
        resolve();
        return;
      }
      reject(new Error(`player exited with code=${code} signal=${signal || "-"}`));
    });
  });
}

async function runSelfTest() {
  const one = {
    trigger: "combat_opening",
    mood: "紧张",
    tts_text: "这手先稳住，别让血线炸开。",
    tts_segments: ["这手先稳住。", "别让血线炸开。"],
  };
  const two = {
    trigger: "reward_choice",
    mood: "克制",
    tts_text: "这张牌不花，但现在它最救命。",
    tts_segments: ["这张牌不花。", "但现在它最救命。"],
  };

  log("info", "self-test starting", { dryRun: config.dryRun });
  enqueue(normalizeTestIntent(one));
  await sleep(50);
  enqueue(normalizeTestIntent(two));
  await sleep(2200);
  log("info", "self-test finished", {});
}

function normalizeTestIntent(beat) {
  return {
    id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    trigger: String(beat.trigger || "beat"),
    mood: String(beat.mood || ""),
    commentary: "",
    segments: normalizeSegments(beat),
    segmentIndex: 0,
    priority: triggerPriority(String(beat.trigger || "beat")),
    createdAt: Date.now(),
  };
}

function triggerPriority(trigger) {
  switch (String(trigger || "").trim()) {
    case "game_over":
      return 100;
    case "combat_opening":
      return 80;
    case "event_choice":
    case "reward_choice":
    case "shop_choice":
      return 70;
    case "card_selection":
    case "rest_choice":
    case "chest_choice":
      return 60;
    case "map_choice":
      return 50;
    default:
      return 40;
  }
}

function sanitizeFileSlug(value) {
  return String(value || "beat")
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, "-")
    .replace(/^-+|-+$/g, "") || "beat";
}

function clampInt(value, fallback, min, max) {
  const parsed = Number.parseInt(String(value || ""), 10);
  if (Number.isNaN(parsed)) {
    return fallback;
  }
  return Math.max(min, Math.min(max, parsed));
}

function clampFloat(value, fallback, min, max) {
  const parsed = Number.parseFloat(String(value || ""));
  if (Number.isNaN(parsed)) {
    return fallback;
  }
  return Math.max(min, Math.min(max, parsed));
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function stripBOM(text) {
  return String(text || "").replace(/^\uFEFF/, "");
}

function log(level, message, data) {
  const line = JSON.stringify({
    time: new Date().toISOString(),
    level,
    message,
    ...data,
  });
  fs.appendFileSync(logPath, line + os.EOL, "utf8");
}

function shutdown() {
  stopActive();
  process.exit(0);
}
