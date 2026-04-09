package main

import "testing"

func TestParsePlayArgsParsesMaxCycles(t *testing.T) {
	headless, attempts, maxCycles, err := parsePlayArgs([]string{"--headless", "--attempts", "0", "--max-cycles", "0"})
	if err != nil {
		t.Fatalf("parsePlayArgs() error = %v", err)
	}
	if !headless {
		t.Fatal("expected headless to be true")
	}
	if attempts != 0 {
		t.Fatalf("attempts = %d, want 0", attempts)
	}
	if maxCycles != 0 {
		t.Fatalf("maxCycles = %d, want 0", maxCycles)
	}
}

func TestParsePlayArgsRejectsNegativeMaxCycles(t *testing.T) {
	if _, _, _, err := parsePlayArgs([]string{"--max-cycles", "-1"}); err == nil {
		t.Fatal("expected parsePlayArgs() to reject negative max cycles")
	}
}
