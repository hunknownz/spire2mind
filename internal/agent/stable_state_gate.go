package agentruntime

import (
	"context"
	"strings"
	"time"

	"spire2mind/internal/game"
)

type StableStateGate struct {
	client *game.Client
}

func NewStableStateGate(client *game.Client) *StableStateGate {
	return &StableStateGate{client: client}
}

func (g *StableStateGate) ReadStableActionableState(ctx context.Context, initial *game.StateSnapshot, timeout time.Duration) (*game.StateSnapshot, error) {
	if g == nil || g.client == nil {
		return initial, nil
	}

	state := initial
	if state == nil {
		var err error
		state, err = g.client.GetState(ctx)
		if err != nil {
			return nil, err
		}
	}
	if game.IsActionableState(state) {
		stabilized, err := g.client.StabilizeActionableState(ctx, state)
		if err == nil && game.IsActionableState(stabilized) {
			return stabilized, nil
		}
		if err != nil && ctx.Err() != nil {
			return nil, err
		}
	}

	if strings.EqualFold(state.Screen, "UNKNOWN") {
		return state, nil
	}

	return g.client.WaitUntilActionable(ctx, timeout)
}

func (g *StableStateGate) FreshActionableStateForExecution(ctx context.Context, candidate *game.StateSnapshot, timeout time.Duration) (*game.StateSnapshot, error) {
	if g == nil || g.client == nil {
		return candidate, nil
	}
	if candidate == nil {
		state, err := g.client.GetState(ctx)
		if err != nil {
			return nil, err
		}
		candidate = state
	}
	if !game.IsActionableState(candidate) {
		waitCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return g.client.WaitUntilActionable(waitCtx, timeout)
	}

	stabilized, err := g.client.StabilizeActionableState(ctx, candidate)
	if err == nil && stabilized != nil && game.IsActionableState(stabilized) {
		return stabilized, nil
	}
	if err != nil && ctx.Err() != nil {
		return nil, err
	}
	return candidate, nil
}

func (g *StableStateGate) StabilizeExecutionDrift(ctx context.Context, expectedState *game.StateSnapshot, liveState *game.StateSnapshot, timeout time.Duration) (*game.StateSnapshot, string) {
	driftKind := classifyStateDrift(expectedState, liveState)
	if expectedState == nil || liveState == nil || g == nil || g.client == nil {
		return liveState, driftKind
	}
	if shouldForceExecutionReread(driftKind) {
		if rereadState, err := g.client.GetState(ctx); err == nil && rereadState != nil {
			liveState = rereadState
			driftKind = classifyStateDrift(expectedState, liveState)
		}
	}
	if !shouldStabilizeExecutionDrift(driftKind) {
		return liveState, driftKind
	}
	if !game.IsActionableState(liveState) {
		if actionableState, err := g.client.WaitUntilActionable(ctx, timeout); err == nil && actionableState != nil {
			liveState = actionableState
			driftKind = classifyStateDrift(expectedState, liveState)
		}
	}

	stabilized, err := g.client.StabilizeActionableState(ctx, liveState)
	if err != nil || stabilized == nil {
		return liveState, driftKind
	}

	return stabilized, classifyStateDrift(expectedState, stabilized)
}
