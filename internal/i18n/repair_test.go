package i18n

import "testing"

func TestRepairTextRepairsUTF8AsGBKMojiBake(t *testing.T) {
	input := "寮€濮嬫垨缁х画褰撳墠鍗曚汉璺戝眬銆?"
	got := RepairText(input)
	if got != "开始或继续当前单人跑局。" {
		t.Fatalf("RepairText() = %q, want %q", got, "开始或继续当前单人跑局。")
	}
}

func TestRepairTextLeavesASCIIUntouched(t *testing.T) {
	input := "Action completed."
	got := RepairText(input)
	if got != input {
		t.Fatalf("RepairText() = %q, want %q", got, input)
	}
}
