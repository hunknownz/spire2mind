package agentruntime

import (
	"errors"
	"fmt"
	"strings"

	"spire2mind/internal/game"
)

const (
	driftKindUnavailable          = "unavailable"
	driftKindRunChanged           = "run_changed"
	driftKindScreenTransition     = "screen_transition"
	driftKindSelectionSeam        = "selection_seam"
	driftKindRewardTransition     = "reward_transition"
	driftKindSameScreenIndexDrift = "same_screen_index_drift"
	driftKindActionWindowChanged  = "action_window_changed"
	driftKindSameScreenStateDrift = "same_screen_state_drift"
)

type stateUnavailableDriftError struct {
	driftKind        string
	message          string
	recoveryReported bool
}

func (e *stateUnavailableDriftError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func classifyStateDrift(expected *game.StateSnapshot, live *game.StateSnapshot) string {
	switch {
	case expected == nil || live == nil:
		return driftKindUnavailable
	case strings.TrimSpace(expected.RunID) != "" &&
		strings.TrimSpace(live.RunID) != "" &&
		!strings.EqualFold(expected.RunID, live.RunID):
		return driftKindRunChanged
	case !strings.EqualFold(expected.Screen, live.Screen):
		switch {
		case strings.EqualFold(live.Screen, "CARD_SELECTION"):
			return driftKindSelectionSeam
		case strings.EqualFold(live.Screen, "REWARD"):
			return driftKindRewardTransition
		default:
			return driftKindScreenTransition + ":" + strings.ToLower(strings.TrimSpace(live.Screen))
		}
	case rewardTransitionWithinScreen(expected, live):
		return driftKindRewardTransition
	case selectionSeamWithinScreen(expected, live):
		return driftKindSelectionSeam
	case strings.Join(expected.AvailableActions, ",") != strings.Join(live.AvailableActions, ","):
		return driftKindActionWindowChanged
	case decisionStateDigest(expected) != decisionStateDigest(live):
		return driftKindSameScreenIndexDrift
	default:
		return driftKindSameScreenStateDrift
	}
}

func rewardTransitionWithinScreen(expected *game.StateSnapshot, live *game.StateSnapshot) bool {
	if expected == nil || live == nil {
		return false
	}
	if !strings.EqualFold(expected.Screen, "REWARD") || !strings.EqualFold(live.Screen, "REWARD") {
		return false
	}

	ep, lp := expected.Reward, live.Reward
	if ep == nil || lp == nil {
		return ep != lp
	}
	return !strings.EqualFold(ep.Phase, lp.Phase) ||
		!strings.EqualFold(ep.SourceScreen, lp.SourceScreen) ||
		!strings.EqualFold(ep.SourceHint, lp.SourceHint) ||
		ep.PendingCardChoice != lp.PendingCardChoice
}

func selectionSeamWithinScreen(expected *game.StateSnapshot, live *game.StateSnapshot) bool {
	if expected == nil || live == nil {
		return false
	}
	if !strings.EqualFold(expected.Screen, "CARD_SELECTION") || !strings.EqualFold(live.Screen, "CARD_SELECTION") {
		return false
	}

	es, ls := expected.Selection, live.Selection
	if es == nil || ls == nil {
		return es != ls
	}
	return !strings.EqualFold(es.Kind, ls.Kind) ||
		!strings.EqualFold(es.SourceScreen, ls.SourceScreen) ||
		!strings.EqualFold(es.SourceHint, ls.SourceHint) ||
		!strings.EqualFold(es.Mode, ls.Mode) ||
		es.RequiresConfirmation != ls.RequiresConfirmation
}

func stateUnavailableError(expected *game.StateSnapshot, live *game.StateSnapshot) error {
	driftKind := classifyStateDrift(expected, live)
	return &stateUnavailableDriftError{
		driftKind: driftKind,
		message: fmt.Sprintf(
			"state_unavailable: drift_kind=%s: live state changed before action execution (expected %s; live %s)",
			driftKind,
			decisionStateSummary(expected),
			decisionStateSummary(live),
		),
	}
}

func recoverableActionKind(err error) string {
	if err == nil {
		return ""
	}

	var driftErr *stateUnavailableDriftError
	if errors.As(err, &driftErr) {
		return driftErr.driftKind
	}

	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "state_unavailable: drift_kind="):
		prefix := "state_unavailable: drift_kind="
		start := strings.Index(text, prefix)
		if start < 0 {
			return "state_unavailable"
		}
		rest := text[start+len(prefix):]
		if end := strings.Index(rest, ":"); end >= 0 {
			return rest[:end]
		}
		return rest
	case strings.Contains(text, "invalid_target"):
		return "invalid_target"
	case strings.Contains(text, "invalid_action"):
		return "invalid_action"
	default:
		return "recoverable"
	}
}

func isSoftReplanDriftKind(kind string) bool {
	kind = strings.TrimSpace(kind)
	switch {
	case kind == driftKindSelectionSeam,
		kind == driftKindRewardTransition,
		kind == driftKindSameScreenIndexDrift,
		kind == driftKindActionWindowChanged,
		strings.HasPrefix(kind, driftKindScreenTransition+":"):
		return true
	default:
		return false
	}
}

func markSoftReplanReported(err error) error {
	var driftErr *stateUnavailableDriftError
	if errors.As(err, &driftErr) {
		driftErr.recoveryReported = true
	}
	return err
}

func recoveryAlreadyReported(err error) bool {
	var driftErr *stateUnavailableDriftError
	return errors.As(err, &driftErr) && driftErr.recoveryReported
}
