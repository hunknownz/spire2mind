# Bridge Design

## Purpose

The Bridge is a C# mod that runs inside Slay the Spire 2 and turns the live game into a local automation API.
It is the only component allowed to touch game objects directly.

## Endpoints

The Bridge keeps a stable API surface:

- `GET /health`
- `GET /state`
- `GET /state?format=markdown`
- `GET /actions/available`
- `POST /action`
- `GET /events/stream`

All responses use a common envelope:

```json
{ "ok": true, "request_id": "req_...", "data": { ... } }
{ "ok": false, "request_id": "req_...", "error": { "code": "...", "message": "...", "retryable": false } }
```

## State schema

`/state` exposes a single top-level snapshot with stable fields:

- `stateVersion`
- `runId`
- `screen`
- `inCombat`
- `turn`
- `availableActions`
- `session`
- `run`
- `combat`
- `map`
- `selection`
- `reward`
- `event`
- `characterSelect`
- `chest`
- `shop`
- `rest`
- `modal`
- `gameOver`
- `agentView`
- `multiplayer`
- `multiplayerLobby`
- `timeline`

The last three are reserved and currently return `null`.

## Threading

All game reads and writes must occur on the Godot game thread.
The Bridge uses `GameThread.InvokeAsync(...)` to marshal work safely from the HTTP listener to the game thread.

Design rules:

- do not read game objects from listener threads
- do not execute actions outside `GameThread`
- keep frame work bounded to avoid visible hitching

## Screen resolution

The raw active screen is not always trustworthy because stale room nodes can remain visible for a few frames.
The Bridge therefore resolves an effective gameplay screen using:

- the active screen
- visible gameplay nodes
- current room information from `RunState`
- actionable map state when the map is already open

Transitions such as map travel are treated as non-actionable until the next room settles.

## Actions

The Bridge exposes only actions that are legal in the current snapshot.
Current action coverage includes:

- main menu and save flow
- game over continue and return to main menu
- character select and embark
- map navigation
- combat cards and end turn
- reward claims, reward cards, skip, and proceed
- event options
- chest open, relic choice, and proceed
- rest options and proceed
- shop open/close, purchases, card removal, and proceed
- generic deck selection
- modal confirm and dismiss

Every `POST /action` follows the same pattern:

1. validate that the action is currently available
2. validate indexes and targets
3. execute on the game thread
4. wait for the next stable snapshot
5. return the latest state

## SSE

`GET /events/stream` publishes lightweight state transitions for faster waiting:

- `stream_ready`
- `state_changed`
- `screen_changed`
- `available_actions_changed`
- `combat_started`
- `combat_ended`
- `combat_turn_changed`
- `player_action_window_opened`

The Go runtime uses SSE first and falls back to polling when necessary.

## Development signing

This project runs on a machine with enforced Windows App Control.
For local development, the Bridge DLL is:

- signed with the `Spire2Mind Dev Code Signing` certificate
- allowed by a supplemental App Control policy

This is a development-only workflow.
Customer machines should still use trusted code signing or a customer-managed allow policy.

## Reverse-engineering discipline

Bridge work is now treated as an ongoing reverse-engineering subsystem, not a one-off adapter.

In practice that means:

- keep live STS2 findings in [bridge-reverse-engineering.md](/C:/Users/klerc/spire2mind/docs/bridge-reverse-engineering.md)
- split new Bridge code by gameplay domain instead of piling onto convenience files
- prefer helper seams over repeated reflection snippets
- treat long unattended runs as the main way to discover transition bugs
