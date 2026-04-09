# Artifact Policy

## Purpose

This project produces two very different kinds of runtime output:

- cross-run knowledge assets that may be kept and versioned
- disposable logs and run artifacts that should not be committed

Those categories need to stay separate.

## Keep And Version

These files are long-lived knowledge assets.
They summarize what the system has learned across many runs and are safe to keep
through major code changes.

- `scratch/guidebook/guidebook.md`
- `scratch/guidebook/living-codex.json`
- `scratch/guidebook/combat-playbook.md`
- `scratch/guidebook/event-playbook.md`

These are now allowed through `.gitignore` so they can be committed when they
contain useful updated knowledge.

## Disposable Logs

These are run logs, mirrors, and transient state captures.
They should not be committed and can be deleted to start a fresh measurement
window after a large update.

- `scratch/agent-runs/`
- `scratch/manual-runs/`
- `scratch/local-llm-launch.log`
- `scratch/state-live.json`
- `scratch/state-live2.json`

Use the cleanup script to remove them:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\clean-logs.ps1
```

## Local Machine Artifacts

These are not knowledge assets and not normal runtime logs.
They are machine-specific local files and should usually stay local.

- `scratch/wdac/`

Do not treat them as gameplay knowledge.
Do not delete them as part of normal log cleanup unless you are intentionally
rebuilding the local WDAC/App Control setup.

## Safety Rule

The cleanup script intentionally preserves:

- `scratch/guidebook/`
- `scratch/wdac/`

It only removes disposable runtime output.
