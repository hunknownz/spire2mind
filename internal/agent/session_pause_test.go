package agentruntime

import (
	"context"
	"testing"
	"time"
)

func TestSessionPauseResumeState(t *testing.T) {
	session := &Session{
		pauseSignal: make(chan struct{}),
		closeSignal: make(chan struct{}),
		events:      make(chan SessionEvent, 8),
	}

	if session.IsPaused() {
		t.Fatal("expected session to start unpaused")
	}
	if !session.Pause() {
		t.Fatal("expected Pause to change session state")
	}
	if !session.IsPaused() {
		t.Fatal("expected session to be paused")
	}
	if session.Pause() {
		t.Fatal("expected repeated Pause to report no state change")
	}
	if !session.Resume() {
		t.Fatal("expected Resume to change session state")
	}
	if session.IsPaused() {
		t.Fatal("expected session to be resumed")
	}
	if session.Resume() {
		t.Fatal("expected repeated Resume to report no state change")
	}
}

func TestSessionWaitIfPausedBlocksUntilResume(t *testing.T) {
	session := &Session{
		pauseSignal: make(chan struct{}),
		closeSignal: make(chan struct{}),
		events:      make(chan SessionEvent, 8),
	}
	if !session.Pause() {
		t.Fatal("expected session to enter paused state")
	}

	done := make(chan error, 1)
	go func() {
		done <- session.waitIfPaused(context.Background())
	}()

	select {
	case err := <-done:
		t.Fatalf("expected waitIfPaused to block while paused, got %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	if !session.Resume() {
		t.Fatal("expected Resume to wake paused waiters")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected waitIfPaused to return nil after resume, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for waitIfPaused to resume")
	}
}

func TestSessionWaitIfPausedReturnsOnClose(t *testing.T) {
	session := &Session{
		pauseSignal: make(chan struct{}),
		closeSignal: make(chan struct{}),
		events:      make(chan SessionEvent, 8),
	}
	if !session.Pause() {
		t.Fatal("expected session to enter paused state")
	}

	done := make(chan error, 1)
	go func() {
		done <- session.waitIfPaused(context.Background())
	}()

	select {
	case err := <-done:
		t.Fatalf("expected waitIfPaused to block while paused, got %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	session.Close()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected waitIfPaused to stop when session closes")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for waitIfPaused to stop on close")
	}
}
