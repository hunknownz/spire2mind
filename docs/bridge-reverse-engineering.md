# Bridge Reverse Engineering Notes

## Purpose

This document tracks the live reverse-engineering knowledge that shapes the Bridge.
It is intentionally more operational than `bridge-design.md`:

- which Slay the Spire 2 systems the Bridge currently trusts
- where we have seen real transition bugs during unattended runs
- which module owns each edge case
- what still needs deeper symbol-level confirmation in `v0.99.1`

The goal is to keep the Bridge maintainable while we continue discovering the game through real runs.

## Current trusted seams

The Bridge currently leans on these runtime seams as the most useful truth sources:

- `ActiveScreenContext.Instance.GetCurrentScreen()`
- `RunManager.Instance`
- `CombatManager.Instance`
- `LocalContext.GetMe(...)`
- room-local Godot nodes exposed by active room screens

These sources are useful, but none of them are individually reliable enough to expose directly as raw automation truth.
Bridge code should keep normalizing them into stable summaries and action predicates.

## Seam entry template

Every seam we keep long-term should use the same structure:

1. `Symptom`
2. `Contradiction`
3. `True seam`
4. `Probe / experiment`
5. `Fix`
6. `Validation`

That keeps reverse-engineering notes comparable across runs and makes it easier to update both Bridge code and validation docs together.

## Tracked seams

### reward_transition

#### Symptom

- a queued reward action can become stale while the reward screen is still visibly resolving
- the agent used to wake up on a reward-like screen before the reward phase had settled

#### Contradiction

- `screen=REWARD` and `available_actions` looked plausible
- but the next legal operation depended on whether reward claim, card choice, or proceed was actually active

#### True seam

- reward flow is not one stable state; it has at least:
  - claim phase
  - card-choice follow-up
  - settle / proceed transition
- screen identity alone is not enough; reward phase and source metadata matter

#### Probe / experiment

- expose reward phase details in live state
- track `reward.sourceScreen` and `reward.sourceHint`
- tighten actionable settle windows for `REWARD`

#### Fix

- Bridge now emits reward phase details through `RewardSectionBuilder` and `StateSnapshotBuilder`
- runtime waits for a stable actionable reward snapshot before reusing queued actions
- soft reward seams are treated as replan points instead of noisy hard failures

#### Validation

- live `/state` now carries `reward.phase`, `reward.sourceScreen`, and `reward.sourceHint`
- recent guidebook hotspots show `reward_transition` trending down rather than dominating the run
- unattended runs keep progressing `REWARD -> CARD_SELECTION -> MAP` without repeated tool errors

### selection_seam

#### Symptom

- the game can enter a card-selection overlay while the parent room still looks active
- earlier runs dropped into `UNKNOWN`, stale `COMBAT`, or wrong indexed selection actions

#### Contradiction

- screen-level classification suggested the parent room was still authoritative
- but room-local selection nodes had already taken over legal interaction

#### True seam

- STS2 reuses multiple local selection overlays:
  - combat hand selection
  - event follow-up selection
  - rest / smith selection
- these overlays must be promoted into first-class `CARD_SELECTION` state with source metadata

#### Probe / experiment

- inspect room-local selection nodes and mode flags
- compare explicit selection screen types with room-tree holder fallbacks
- validate that selection options come from the owning seam, not a generic grid scan

#### Fix

- `GameUiAccess.ResolveSelectionContext(...)` became the normalization seam
- `SelectionSectionBuilder` and `ScreenClassifier` now prefer explicit selection contexts over parent rooms
- runtime now remaps selection decisions by stable option identity before falling back to raw index

#### Validation

- event follow-up selection in `BRAIN_LEECH` now resolves as `CARD_SELECTION`
- combat-local selection uses mode-specific hand methods instead of generic UI clicks
- recent runs show `selection_seam` as rare instead of dominant

### same_screen_index_drift

#### Symptom

- the screen stays the same, but target/card/reward indexes shift before the queued action lands
- this used to surface as repeated `invalid_action` or target errors

#### Contradiction

- from the model's point of view, it chose a legal action on the current screen
- by execution time, the live screen still matched, but the indexed object order no longer did

#### True seam

- the real instability is not screen identity; it is index identity
- repeated cards, enemies, or reward items can reorder while keeping the same parent screen

#### Probe / experiment

- compare expected vs live state fingerprints before execution
- inspect duplicate-card and duplicate-enemy cases
- track drift kinds separately from provider or tool failures

#### Fix

- runtime normalizes and remaps indexed decisions before execution
- duplicate-aware remap now prefers stable semantic identity and relative ordering over raw index reuse
- same-screen drift can degrade into silent reuse or soft replan instead of hard failure

#### Validation

- guidebook recent hotspots show `same_screen_index_drift` steadily shrinking across recent runs
- direct actions and model-decided actions both use pre-execution remap
- target and option mismatches no longer dominate soak logs

### GAME_OVER boundary

#### Symptom

- after lethal resolution or `continue_after_game_over`, the live state can remain on `GAME_OVER` while still settling
- earlier runtime versions woke the model too early and invented fake actions on dead summary screens

#### Contradiction

- `screen=GAME_OVER` looked terminal and therefore actionable
- but the actual legal follow-up did not exist yet because summary transitions were still in flight

#### True seam

- `GAME_OVER` is a staged rollover flow, not a single state
- the runtime must distinguish:
  - settled summary with real actions
  - transition frames with no actions
  - post-summary return-to-menu handoff

#### Probe / experiment

- track `gameOver.stage`
- enforce settle windows for `GAME_OVER`
- observe whether the next attempt bootstrap happens without spurious intermediate actions

#### Fix

- `internal/game/client.go` no longer treats empty `GAME_OVER` as actionable
- `readActionableState()` stabilizes even already-actionable states before handing them to the model
- attempt lifecycle is kept explicit during summary closeout and next-attempt bootstrap

#### Validation

- recent long soaks have repeatedly reached:
  - `GAME_OVER`
  - `continue_after_game_over`
  - `return_to_main_menu`
  - next attempt bootstrap
- the transition no longer emits noisy fake actions like `end_turn` on dead summary screens

## Known live edge cases

### 1. Map travel ghost actions

Observed behavior:

- after `choose_map_node`, stale room nodes can remain visible for a few frames
- if exposed directly, the agent can act on the previous room while travel is still resolving

Current mitigation:

- `AvailableActionBuilder`
- `StateSnapshotBuilder`

Rule:

- during `map.isTraveling == true`, expose no room actions
- only resume actions once the next room snapshot is settled

### 2. Finished events still require `choose_event_option`

Observed behavior:

- some finished event screens remain classified as `EVENT`
- `event.isFinished == true`
- `event.options == []`
- yet the live flow still expects the equivalent of a proceed action

Current mitigation:

- Bridge keeps exposing `choose_event_option`
- executor path in `BridgeActionExecutor.Rooms.cs` already knows how to treat a finished event as a proceed
- Go-side validation now allows `choose_event_option` without an `option_index` when `event.isFinished == true`
- deterministic and rule-based fallback now emit `choose_event_option` with `option_index=0` for finished events, so provider fallback no longer stalls on this boundary

Future cleanup:

- confirm whether the Bridge should instead surface a canonical `proceed` on finished events
- if we keep `choose_event_option`, document that finished events accept no index or `0`

### 3. Combat action window false negatives

Observed behavior:

- live combat snapshots can show playable cards, energy, and enemies
- but the low-level combat gate occasionally reports no legal actions
- this can stall unattended runs because the runtime waits forever on an empty action list

Current mitigation:

- `CombatActionAvailability`
- `AvailableActionBuilder`

Rule:

- trust strong gameplay signals from the normalized combat snapshot
- if the hand contains playable cards, or the player still has energy and live enemies, expose a combat decision window even if a lower-level UI gate is briefly pessimistic

Why this matters:

- a false positive is usually recoverable through action validation
- a false negative can deadlock unattended play

### 3a. Combat can embed a card-selection overlay

Observed behavior:

- the active room remains `NCombatRoom`
- combat hand cards are still visible
- `Hand.IsInCardSelection == true`
- normal combat actions become invalid even though the raw hand still looks playable

What this means:

- this is not a regular combat decision window
- it is a combat-local selection overlay and should be modeled as `CARD_SELECTION`

Current mitigation:

- `GameUiAccess.ResolveSelectionContext(...)`
- `SelectionSectionBuilder`
- `ScreenClassifier`
- `BridgeActionExecutor.Rooms.cs`

Rule:

- if combat embeds a card-selection state, prefer `CARD_SELECTION` over `COMBAT`
- expose `select_deck_card`, not `play_card/end_turn`
- keep `in_combat=true` and preserve the combat summary for context

Verified `v0.99.1` details:

- the selection lives on `NCombatRoom.Ui.Hand`
- the hand enters selection with `IsInCardSelection == true`
- actionable modes are `SimpleSelect` and `UpgradeSelect`
- visible options come from `ActiveHolders`, not from a generic screen-level card grid
- the header text can be read from `%SelectionHeader`
- selection execution should call:
  - `SelectCardInSimpleMode(...)` or
  - `SelectCardInUpgradeMode(...)`
  - then `CheckIfSelectionComplete()`
- selection prefs come from the hand's `_prefs`
- already chosen entries can be counted via `_selectedCards`

### 4. Structured decision payloads can include stale optional indexes

Observed behavior:

- the model may emit a technically unnecessary `target_index` or `option_index`
- Bridge execution can reject those extra parameters even when the base action is still correct

Current mitigation:

- Go runtime now normalizes action requests against the current state before executing them
- example: non-targeted cards have `target_index` stripped automatically

Owning module:

- `internal/agent/decision.go`

### 5. Empty `GAME_OVER` is a transition, not an actionable state

Observed behavior:

- after `continue_after_game_over`, the live state can remain on `GAME_OVER`
- `available_actions == []`
- a second or later summary step eventually exposes `return_to_main_menu`

What went wrong:

- one runtime layer still treated `screen == GAME_OVER` as automatically actionable
- that let the model wake up and invent fake actions like `end_turn` on a dead summary screen

Current mitigation:

- `internal/game/client.go`
- `Session.readActionableState(...)`

Rule:

- a state is only actionable when it exposes real actions
- `GAME_OVER` with an empty action list is still settling and should stay inside the wait loop

Why this matters:

- these false wakeups create noisy `invalid_action` loops at the exact moment we want clean attempt rollover
- keeping the wait predicate strict makes multi-attempt unattended runs more stable

## Engineering rules for Bridge maintenance

### Split by gameplay domain, not by convenience

When Bridge logic grows, split it by domain:

- screen classification
- section builders
- action availability
- action executors
- UI access helpers
- reflection helpers or symbol helpers
- wait strategies

Avoid central God files even if that makes the first patch slower.

### Prefer stable-seam helpers over repeated reflection snippets

If a symbol or Godot path is used more than once, promote it into a helper:

- `GameUiAccess`
- `ReflectionUtils`
- screen-specific builder helpers

That keeps reverse-engineering drift localized when the game updates.

### Action execution must always own its own settle logic

Every write path should keep the same shape:

1. validate action availability
2. validate indexes and targets
3. execute on the game thread
4. wait for the next stable snapshot
5. return the newest snapshot

Never rely on the caller to "know" that a room is still transitioning.

### Bias toward recoverable false positives over dead-end false negatives

For unattended play:

- exposing a slightly optimistic action that later fails as `invalid_action` is recoverable
- hiding the only way forward often causes the runtime to stall

This tradeoff should be explicit in combat and transition-heavy screens.

## Documentation backlog

The following areas still need richer `v0.99.1` notes:

- event room variants and their finished/proceed semantics
- merchant internals and inventory-open state transitions
- rest site option flow, especially smith or selection interactions
- treasure screen and relic selection variants
- game-over summary flow and return-to-menu behavior
- detailed combat phase flags that best predict player action windows

## Practical workflow

When a live run finds a new Bridge bug:

1. capture the last `dashboard.md`, `state-latest.json`, and `events.jsonl`
2. identify whether the bug belongs to:
   - screen classification
   - state building
   - action availability
   - action execution
   - settle or wait logic
3. patch the smallest owning module
4. add a short note here if the behavior teaches us something new about STS2

That keeps reverse engineering, engineering quality, and unattended play moving together instead of drifting apart.

## Retrospective: why combat hand selection took too many iterations

This was the `ARMAMENTS -> choose a card to upgrade` bug.

### What initially happened

- the live run entered a combat-local card selection step
- the Bridge still surfaced `screen=COMBAT`
- `available_actions` exposed `play_card,end_turn`
- every attempted action failed as `invalid_action`

At first glance this looked like a combat timing problem, so the first fixes mostly targeted:

- wait behavior
- optimistic combat action inference
- runtime retries

Those changes improved observability, but they did not solve the root cause.

### What the actual root cause was

The state was not a normal combat decision window at all.

It was:

- still inside `NCombatRoom`
- but the hand had switched into a selection sub-mode
- `Ui.Hand.IsInCardSelection == true`
- `Ui.Hand.CurrentMode` was no longer regular play

So the Bridge had a modeling bug:

- state classification said `COMBAT`
- executor validation correctly refused normal combat actions
- the two layers were working from different truths

### Why it took multiple attempts

#### 1. I treated it as a timing problem before proving it was a modeling problem

The first instinct was:

- maybe the state is stale
- maybe the action window reopened late
- maybe polling or SSE is missing a transition

That was only partially useful.
The real question should have been:

- why does `available_actions` say the action is legal while the executor says it is not?

That is almost always a classification or predicate mismatch, not just a wait bug.

#### 2. I fixed the outside of the system before reconciling the inside

I improved:

- fallback combat availability
- runtime replanning on stale state
- long-run retries
- dashboard or debug output

Those were good investments, but they came before the core invariant was restored.
The system invariant should have been checked first:

- the state builder
- the availability builder
- the executor

must describe the same underlying gameplay mode.

#### 3. I used a generic UI interaction where STS2 actually had a mode-specific interaction

Once the state was correctly classified as `CARD_SELECTION`, the next bug remained:

- `select_deck_card` still did not work

Why?

Because I initially reused the generic selection interaction:

- `selected.EmitSignal(NCardHolder.SignalName.Pressed, selected)`

That works for many standalone selection screens, but not for combat-hand selection.
The real seam was mode-specific:

- `NCombatRoom.Ui.Hand.ActiveHolders`
- `SelectCardInSimpleMode(...)`
- `SelectCardInUpgradeMode(...)`
- `CheckIfSelectionComplete()`

Until I switched to those hand-specific methods, the game never logged a chosen card.

#### 4. I should have compared against a known-good reference sooner, but only after narrowing the seam

`STS2-Agent` already had a combat-hand selection path.
The right moment to consult it was after the problem had been reduced to:

- "combat-local selection exists"
- "generic selection click does not resolve it"

At that point the reference became highly valuable.
Using it earlier would have risked cargo-culting; using it at the narrowed seam was the right move.

### What finally solved it

The successful sequence was:

1. add executor diagnostics so invalid actions reported low-level combat state
2. observe `IsInCardSelection == true`
3. promote combat-hand selection to a first-class selection context
4. classify the live state as `CARD_SELECTION`, not `COMBAT`
5. expose `select_deck_card` instead of `play_card/end_turn`
6. execute combat-hand selection through `NPlayerHand` mode-specific methods

The proof that the fix was real was not just the HTTP state shape.
The proof was the game log:

- `Player 1 chose cards [STRIKE_IRONCLAD]`

That meant the Bridge was finally speaking the game's actual interaction language.

### 3b. Event follow-up card selection can live in the room tree, not only in `IScreenContext`

Observed behavior:

- `choose_event_option` in `BRAIN_LEECH` used to return a stable snapshot with:
  - `screen=UNKNOWN`
  - `available_actions=[]`
- the run was not actually stuck
- the game had transitioned into a card-pick overlay, but the Bridge stopped seeing it

What was misleading:

- our selection seam only trusted an allow-list of deck-selection screen type names
- the live event follow-up selection came through `NSimpleCardSelectScreen`, which was a real screen but not yet on that allow-list
- after the first fix, a too-broad grid-holder fallback also started pulling reward card choice into `CARD_SELECTION`

Current mitigation:

- `GameUiAccess.ResolveSelectionContext(...)`
- `GameUiAccess.GetVisibleGridCardHolders(...)`
- `ScreenClassifier`
- `SelectionSectionBuilder`

Rule:

- explicit selection screen types must stay on a maintained allow-list
- `NSimpleCardSelectScreen` is now treated as a first-class deck-selection screen
- visible `NGridCardHolder`s remain a fallback seam, but dedicated non-deck flows like reward screens must be excluded
- selection options should come from visible grid holders before generic `NCardHolder` scans

Verified `v0.99.1` proof:

- `BRAIN_LEECH -> 分享知识`
- Bridge now returns:
  - `screen=CARD_SELECTION`
  - `available_actions=[select_deck_card]`
  - 5 concrete card options
- `selection.kind=NSimpleCardSelectScreen`
- after `select_deck_card`, Bridge returns the finished event
- after a final `choose_event_option`, Bridge returns `MAP`

Why this matters:

- this is another example of STS2 introducing meaningful screen types outside our original allow-list
- if we only trust a stale list of known screen names, unattended play will keep dropping into `UNKNOWN`
- if we overcorrect with a blanket grid-holder fallback, we blur deck selection with reward selection
- future selection bugs should first ask:
  - is this actually a new explicit selection screen type?
  - if not, is there a room-local holder seam worth using as a fallback?

## Next-time debugging checklist

When a similar reverse-engineering bug appears, follow this order:

1. Check invariants first.
- Does `available_actions` agree with executor validation?
- If not, stop adding retries and compare the owning predicates.

2. Promote diagnostics before heuristics.
- Add low-level detail to the failing action path.
- Learn which hidden mode flag is active before broadening availability.

3. Model the state correctly before improving waits.
- If the state is really a sub-mode, give it a first-class section or screen.
- Do not keep pretending it is the parent room state.

4. Prefer game-native seams over generic UI clicks.
- If the game has a room-specific method, use it.
- Generic button or card signals are fallback tools, not the first choice.

5. Use references only after narrowing the seam.
- First discover what kind of bug it is on our own build.
- Then compare with `STS2-Agent` or other references at that exact seam.

6. Write the result down immediately.
- Record the hidden flags, nodes, methods, and failure pattern here.
- That turns a frustrating bug into permanent reverse-engineering capital.

## Reverse-engineering playbook

When a live automation bug appears, do not jump straight into retries or heuristics.
Walk it down in layers:

### 1. Name the failing invariant

Before changing code, write the exact contradiction in one sentence.

Examples:

- `available_actions` says `play_card`, but executor rejects `play_card`
- screen says `COMBAT`, but a room-local node says selection is active
- action returns success, but the next stable snapshot is unchanged

If the contradiction is not named clearly, fixes tend to drift toward symptoms.

### 2. Decide which layer is lying

Every Bridge bug should be localized to one owning layer first:

- screen classification
- section building
- action availability
- action execution
- settle or wait logic
- runtime replanning

Do not patch multiple layers at once until one layer is proven wrong.
Otherwise we create compensating bugs that hide the real seam.

### 3. Prefer evidence from the failing write path

If an action fails, the write path usually has the most useful truth.
Add diagnostics there first:

- hidden mode flags
- node type names
- counts of visible or selectable items
- room-local state booleans
- validation reason for rejection

That is usually more valuable than adding more polling or more retries.

### 4. Distinguish bug class before tuning waits

Most automation bugs fall into one of these buckets:

- modeling bug: the state shape is wrong
- execution bug: the action path is wrong
- settle bug: the action succeeded but the wait logic observed the wrong follow-up state

Only the third class should start with wait tuning.
The first two need better modeling or a different game-native seam.

### 5. Promote hidden sub-modes into first-class state

If a room can locally enter a different interaction mode, model that explicitly.

Examples:

- combat-local card selection
- reward-local card selection
- smith or upgrade selection
- modal overlays over normal room state

Treating sub-modes as "still basically the parent room" is one of the easiest ways to build action mismatches.

### 6. Reach for references only after narrowing the seam

Reference projects are most valuable when the question is already precise.

Bad timing:

- copying their room model before proving our room model is the problem

Good timing:

- "we now know this is embedded combat card selection; how does a known-good implementation trigger the choice?"

That keeps references from turning into cargo-cult patches.

## Experiment log template

For future Bridge bugs, log experiments in this shape before or during the fix:

1. Symptom
- what the player or agent saw
- what action failed

2. Contradiction
- which two layers disagree

3. Hypothesis
- what hidden mode, node, or timing edge might explain the disagreement

4. Probe
- what diagnostic was added or what exact runtime value was inspected

5. Result
- what the probe proved false
- what it proved true

6. Fix
- which owning module changed
- why that module was the right place

7. Proof
- game log line, state transition, or repeated unattended success that shows the fix is real

This helps us avoid "many small retries with no memory" and turns each hard bug into a reusable reverse-engineering pattern.
