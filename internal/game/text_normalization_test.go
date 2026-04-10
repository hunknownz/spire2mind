package game

import "testing"

func TestNormalizeStateSnapshotRepairsNestedStrings(t *testing.T) {
	state := &StateSnapshot{
		RunID:            "RUN-1",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "鍔ㄤ綔"},
		Combat: map[string]any{
			"enemyName": "鎵撳嚮",
			"cards": []any{
				map[string]any{"name": "闃插尽"},
			},
		},
	}

	got := NormalizeStateSnapshot(state)
	if got.AvailableActions[1] != "动作" {
		t.Fatalf("expected repaired action name, got %q", got.AvailableActions[1])
	}
	if got.Combat["enemyName"] != "打击" {
		t.Fatalf("expected repaired enemy name, got %#v", got.Combat["enemyName"])
	}
	cards := got.Combat["cards"].([]any)
	card := cards[0].(map[string]any)
	if card["name"] != "防御" {
		t.Fatalf("expected repaired card name, got %#v", card["name"])
	}
}
