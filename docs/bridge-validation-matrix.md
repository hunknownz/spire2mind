# Bridge Validation Matrix

## Purpose

This document tracks the seams and transition-heavy flows that matter most for unattended play.
It complements `bridge-reverse-engineering.md`:

- `bridge-reverse-engineering.md` explains what the seam is and how we found it
- this file tracks what has actually been validated on recent builds

The matrix should only describe flows that have real run evidence behind them.

## Status scale

- `Stable`: repeatedly validated in unattended runs on the current build
- `Hardened`: fixed and validated, but still worth watching in recent guidebook trends
- `Active hotspot`: mitigated, but still one of the top remaining recovery sources

## Seam matrix

| Seam / flow | Symptom | Owning modules | Current status | Validation notes |
| --- | --- | --- | --- | --- |
| `reward_transition` | reward claim / card choice / proceed can reorder while the screen still looks like `REWARD` | `RewardSectionBuilder`, `StateSnapshotBuilder`, `internal/game/client.go`, `internal/agent/session.go` | Hardened | live state carries reward phase and source metadata; recent runs still see occasional soft replans, but no longer frequent tool errors |
| `selection_seam` | selection overlays can hide inside combat, event, or rest flows | `GameUiAccess.ResolveSelectionContext`, `SelectionSectionBuilder`, `ScreenClassifier`, `BridgeActionExecutor.Rooms.cs` | Hardened | combat-hand and event follow-up selection are validated; recent guidebook trends show this seam as rare |
| `same_screen_index_drift` | screen remains the same while indexed targets or options reorder | `internal/agent/decision_remap.go`, `internal/agent/decision.go`, `internal/agent/session.go`, `internal/agent/statefingerprint.go` | Active hotspot | duplicate-aware remap and pre-execution normalization are working, but this remains the largest residual recovery source |
| `action_window_changed` | combat action window closes during execution or settle | `CombatActionAvailability`, `AvailableActionBuilder`, `internal/game/client.go` | Hardened | recent runs mostly absorb this as soft replan instead of hard failure |
| `GAME_OVER boundary` | dead summary screens used to wake the model too early | `internal/game/client.go`, `internal/agent/session.go` | Stable | recent multi-attempt soaks have repeatedly crossed `GAME_OVER -> return_to_main_menu -> next attempt` on the current runtime |

## Flow validation checklist

### Combat and reward chain

- `COMBAT -> REWARD`
- `REWARD -> CARD_SELECTION`
- `CARD_SELECTION -> REWARD`
- `REWARD -> MAP`

Current status:

- validated on recent unattended runs
- still worth watching for `reward_transition` and `same_screen_index_drift`

### Event selection chain

- `EVENT -> choose_event_option`
- `EVENT(isFinished=true, options=[]) -> choose_event_option`
- `EVENT -> CARD_SELECTION`
- `CARD_SELECTION -> EVENT`
- `EVENT -> MAP`

Current status:

- validated for known event follow-up selection seams
- finished event screens are still represented as `EVENT`, but the runtime now treats `choose_event_option` with no options as the canonical proceed path
- explicit selection screen typing must remain maintained as new event variants appear

## Automated invariant coverage

The runtime now keeps a small invariant suite in Go tests for the highest-value contracts:

- targeted `play_card` requests must satisfy the live `target_index` contract
- finished `EVENT` states must accept `choose_event_option` as the continuation action
- `REWARD`, `CARD_SELECTION`, and `GAME_OVER` boundary states must respect actionable/non-actionable semantics

These tests live alongside the runtime so regressions are caught before the next unattended soak.

The default entrypoint for these checks is:

- `scripts/test-state-invariants.ps1`

## Contract matrix

The rows below are the executable contract view of the seam matrix. The goal is to move
state/action semantics forward into Bridge payloads and invariant tests so the runtime has
less guesswork and fewer recovery-only fixes.

| Contract | Expected state fields | Expected available actions | Automated coverage | Recent hotspot impact | Next action |
| --- | --- | --- | --- | --- | --- |
| `available_actions` sync | `state.availableActions` must describe the same legal set as `/actions/available` for the live screen | no stale or missing action ids across the two endpoints | `scripts/test-state-invariants.ps1`; `internal/game/state_invariants_test.go` | feeds every hotspot when stale actions leak through | add explicit bridge-side parity checks and live smoke assertions |
| `combat target contract` | `combat.hand[].requiresTarget`, `targetType`, `validTargetIndices`, hittable enemies | targeted `play_card` must require `target_index`; untargeted cards must not | `TestActionInvariantTargetedPlayableCardRequiresValidTargetIndex`; `TestExecuteDirectActionRejectsTargetedPlayCardWithoutTargetBeforeBridgeCall` | controls invalid target failures and a slice of `same_screen_index_drift` | extend live contract coverage for duplicate enemies and retarget after kill |
| `combat action window contract` | `combat.actionWindowOpen`, enemy hittability, end-turn-only late-turn edge state | only expose `play_card` / `end_turn` when the player can really act | `TestStateInvariantCombatActionWindowClosedIsNotActionable`; `TestStateInvariantCombatActionWindowOpenIsActionable`; `TestWaitUntilActionable*` | primary driver of `action_window_changed` | keep tightening Bridge action exposure and settle windows for transition frames |
| `reward main screen contract` | `reward.phase`, `pendingCardChoice`, `claimable rewards`, `canProceed`, `sourceScreen`, `sourceHint` | `claim_reward` while claimables remain; `proceed` only after rewards are truly resolved | `TestStateInvariantRewardClaimPhaseIsActionable`; `TestStateInvariantRewardProceedPhaseIsActionable`; `TestActionInvariantProceedIsBlockedWhileClaimableRewardsRemain`; `TestActionInvariantProceedAllowsSettledRewardScreen` | primary driver of `reward_transition` | keep `proceed` suppressed until claim/card-choice work is finished and verify on live reward chains |
| `reward card overlay contract` | `reward.phase=card_choice`, `pendingCardChoice`, `cardOptions[]` | `choose_reward_card` / `skip_reward_cards`; no premature `proceed` | `TestActionInvariantPendingRewardChoiceRequiresIndexedCardOption`; `TestActionInvariantProceedIsBlockedWhileSelectionChoiceRemains` | secondary `reward_transition` source | add live overlay parity checks and reward card evidence to the matrix |
| `selection source contract` | `selection.kind`, `sourceScreen`, `sourceHint`, `requiresConfirmation`, `canConfirm`, `cards[]` | `select_deck_card` only when a real deck selection is active; expose `confirm_selection` when the UI requires explicit confirmation | `TestStateInvariantSelectionWithDeckChoiceIsActionable`; `TestStateInvariantSelectionConfirmPhaseIsActionable`; `TestActionInvariantProceedIsBlockedWhileSelectionChoiceRemains`; `TestActionInvariantConfirmSelectionRequiresConfirmationContract`; `TestActionInvariantProceedIsBlockedWhileSelectionConfirmationRemains` | explains the remaining `selection_seam` and part of `same_screen_index_drift` | extend matrix rows by source type: rest, event, shop remove, combat embedded; keep combat-hand confirm paths validated |
| `event transform selection contract` | `selection.kind=NDeckTransformSelectScreen`, `minSelect`, `maxSelect`, `selectedCount`, `cards[].isSelected`, `requiresConfirmation`, `canConfirm` | continue exposing `select_deck_card` while the required picks are incomplete, then switch to `confirm_selection` once the transform preview can be confirmed | `TestChooseDeterministicActionSelectsNextUnselectedDeckCard`; `TestChooseDeterministicActionConfirmsDeckSelectionWhenMinimumReached`; live event transform replay evidence | repeated `select_deck_card` loops can escalate into Bridge disconnects if this contract regresses | keep the transform screen on the invariant checklist and verify it after any selection builder or executor change |
| `simple combat selection contract` | `selection.kind=NSimpleCardSelectScreen`, `sourceScreen=COMBAT`, `requiresConfirmation=false`, `canConfirm=false`, `selectedCount` | expose only `select_deck_card`; never expose `confirm_selection` | `TestStateInvariantSimpleSelectionDoesNotRequireConfirmation`; live run `20260408-200539-19429e5d-ffa5-4c74-9691-b28ab49f6a04` | this was the direct root cause of the recent `select_deck_card -> internal_error` | keep simple selection hard-coded to no-confirm in Bridge and re-verify after any selection context changes |
| `reward overlay exclusivity contract` | `reward.pendingCardChoice=true`, `reward.cardOptions[]`, optional `selection.kind=reward_cards` | only `choose_reward_card` / `skip_reward_cards`; never mix with `select_deck_card` or `confirm_selection` | `TestStateInvariantRewardCardOverlayIsActionableWithoutDeckSelectionAction`; `TestStateInvariantRewardCardOverlayRejectsDeckSelectionAction` | mixed action sets previously routed reward picks into the generic deck-selection executor | keep reward overlay parity checks in both Bridge action building and runtime actionable gating |
| `reward claim/proceed exclusivity contract` | `reward.rewards[].claimable`, `reward.canProceed`, `reward.phase` | do not expose `proceed` while any claimable reward remains; do not expose `claim_reward` once only proceed remains | `TestStateInvariantRewardClaimPhaseRejectsProceedWhileClaimablesRemain`; `TestActionInvariantProceedIsBlockedWhileClaimableRewardsRemain`; live reward chain checks | directly influences `reward_transition` and stale reward actions | add more live reward evidence after unattended soaks and keep proceed suppression strict |
| `finished EVENT continuation contract` | `event.isFinished`, `event.options=[]` | `choose_event_option` remains the canonical continue path | `TestStateInvariantFinishedEventContinuationIsActionable`; `TestActionInvariantFinishedEventAllowsChooseEventOptionZero`; `TestChooseDeterministicActionAdvancesFinishedEvent`; `TestChooseRuleBasedActionAdvancesFinishedEventSingleAction` | prevents model-unavailable stalls on finished events | keep validating as new event variants appear |
| `GAME_OVER rollover contract` | `gameOver.stage`, `canContinue`, `canReturnToMainMenu` | no actions during transition; allow `continue_after_game_over` / `return_to_main_menu` only after settle | `TestStateInvariantGameOverTransitionIsNotActionable`; `TestStateInvariantGameOverContinueStageIsActionable`; `TestWaitUntilActionableDoesNotTreatEmptyGameOverAsActionable` | critical for clean unattended multi-attempt loops | keep revalidating after any wait-loop or settle-window change |

## Runtime refactor acceptance

These rows track the new runtime subsystems that are replacing scattered patch logic.
They are not just implementation notes; each row should be backed by tests and recent run evidence.

| Subsystem | Scope | Current expectation | Verification |
| --- | --- | --- | --- |
| `ActionResolutionPipeline` | unify `ActionDecision -> ActionRequest`, remap, normalize, validate | `play_card` must never leak `option_index`; reward/selection/shop/event option semantics stay separated from combat target semantics | `internal/agent/action_resolution_test.go`; `internal/agent/decision_test.go`; targeted runtime regressions in `internal/agent/session_test.go` |
| `StableStateGate` | unify fresh/stable/actionable reads before execution | runtime should consume a single stable actionable state abstraction instead of scattered wait/settle logic | `internal/game/state_invariants_test.go`; `internal/game/client_test.go`; `scripts/test-state-invariants.ps1` |
| `PromptAssemblyPipeline` | replace full-state prompt accretion with screen-scoped blocks | prompt size must decrease while keeping legal-action accuracy | prompt size before/after snapshots; cycle telemetry once usage logging lands |
| `Reasoning/Reflection Pipeline` | split live reasoning from attempt reflection | live reasoning must not contain tool-call meta narration; attempt reflections must classify tactical/runtime/resource failures separately | cycle transcript review; reflection structure tests once the new pipeline lands |

## Next implementation order

1. Keep turning `action_window_changed` into a protocol problem first, not a runtime-only recovery.
2. Continue tightening `reward_transition` by suppressing premature `proceed` and documenting live evidence.
3. Finish replacing scattered action normalization with `ActionResolutionPipeline`.
4. Expand the contract rows above with live run evidence after each unattended soak.
5. Use guidebook hotspots only as the prioritization signal; the durable fix should land in Bridge semantics plus invariant coverage.

## Borrowed from STS2-Agent

These are the specific contract lessons we are continuing to copy from
`research/STS2-Agent`, because they map directly onto our remaining hotspots:

- reward main screen and reward card overlay must be modeled as distinct phases, not as one loose `REWARD` bucket
- `available_actions` must stay aligned with the payload fields that justify them; stale or mixed action sets are contract bugs first
- selection flows need per-kind rules, especially around `requiresConfirmation` and `canConfirm`, instead of letting runtime infer them ad hoc
- validation should be checklist-driven after Bridge changes, especially for reward, selection, and game-over seams

### Game over rollover

- `GAME_OVER -> continue_after_game_over`
- `GAME_OVER -> return_to_main_menu`
- `MAIN_MENU -> next attempt bootstrap`

Current status:

- validated in recent multi-attempt soaks
- should keep being rechecked after any wait-loop or settle-window changes

## Update rules

When a seam changes state:

1. update the seam entry in `bridge-reverse-engineering.md`
2. update the row in this matrix
3. note the run evidence that justified the change

Do not mark a seam `Stable` based on a one-off local repro fix.
It should survive unattended runs on the current build first.
