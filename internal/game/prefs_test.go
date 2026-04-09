package game

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureFastModeUpdatesPrefsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "prefs.save")
	if err := os.WriteFile(path, []byte("{\n  \"fast_mode\": \"normal\",\n  \"show_run_timer\": false\n}\n"), 0o644); err != nil {
		t.Fatalf("write prefs: %v", err)
	}

	status, err := EnsureFastMode(path, "instant")
	if err != nil {
		t.Fatalf("ensure fast mode: %v", err)
	}
	if !status.Changed {
		t.Fatalf("expected fast mode change to be recorded")
	}
	if status.Previous != "normal" || status.Current != "instant" {
		t.Fatalf("unexpected status: %#v", status)
	}

	readBack, err := ReadFastMode(path)
	if err != nil {
		t.Fatalf("read fast mode: %v", err)
	}
	if readBack.Current != "instant" {
		t.Fatalf("expected persisted fast mode instant, got %q", readBack.Current)
	}
}

func TestEnsureFastModeNoopWhenAlreadySet(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "prefs.save")
	if err := os.WriteFile(path, []byte("{\n  \"fast_mode\": \"instant\"\n}\n"), 0o644); err != nil {
		t.Fatalf("write prefs: %v", err)
	}

	status, err := EnsureFastMode(path, "instant")
	if err != nil {
		t.Fatalf("ensure fast mode: %v", err)
	}
	if status.Changed {
		t.Fatalf("expected no change when fast mode already matches")
	}
	if status.Current != "instant" || status.Previous != "instant" {
		t.Fatalf("unexpected status: %#v", status)
	}
}
