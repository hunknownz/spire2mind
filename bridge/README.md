# Spire2Mind Bridge

This directory contains the live Slay the Spire 2 Bridge mod.

## Current scope

The Bridge is no longer read-only.
It currently supports:

- `GET /health`
- `GET /state`
- `GET /state?format=markdown`
- `GET /actions/available`
- `POST /action`
- `GET /events/stream`

Implemented room and action coverage includes:

- main menu and save flow
- character select and embark
- map navigation
- combat
- rewards and card rewards
- events
- chest rooms
- rest sites
- shop flow
- generic card selection
- modal dialogs

## Layout

- `Entry.cs`, `BridgeRuntime.cs`
  Bridge bootstrap and lifecycle.
- `Models/`
  Stable state and API DTOs split by gameplay area.
- `game/`
  Screen resolution, state builders, action executors, thread dispatch, and formatting.
- `server/`
  HTTP routing, SSE event service, and JSON helpers.
- `scripts/`
  Build, signing, install, smoke, room coverage, and App Control helpers.

## Build

```powershell
pwsh .\bridge\scripts\build.ps1
```

## Install

```powershell
pwsh .\bridge\scripts\install.ps1
```

`install.ps1` requires the game process to be closed so the DLL can be replaced cleanly.

## Development signing

The Bridge DLL is signed during build and install.

- `scripts/ensure-dev-codesign-cert.ps1`
  Creates the local `CN=Spire2Mind Dev Code Signing` certificate when needed.
- `scripts/sign-bridge-dll.ps1`
  Signs `Spire2Mind.Bridge.dll`.

## Development App Control policy

On this development machine, Windows App Control blocks unsigned or untrusted DLLs.
The repo includes a supplemental allow-policy workflow for the dev signing certificate:

```powershell
pwsh .\bridge\scripts\new-dev-appcontrol-supplemental-policy.ps1
pwsh .\bridge\scripts\install-dev-appcontrol-policy.ps1
```

This is only a local development workaround.
Customer distribution should still use trusted code signing or a managed allow policy.

## Smoke tests

- Bridge-only smoke:

```powershell
pwsh .\bridge\scripts\smoke-test.ps1
```

- Multi-room validation:

```powershell
pwsh .\bridge\scripts\cover-rooms.ps1
```
