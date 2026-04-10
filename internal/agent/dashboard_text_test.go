package agentruntime

import "testing"

func TestCompactTextPreservesUTF8Boundaries(t *testing.T) {
	input := "铁甲战士 对阵 小啮兽"

	got := compactText(input, 8)
	if got == input {
		t.Fatalf("expected truncation, got full text %q", got)
	}
	if got != "铁甲战士 ..." {
		t.Fatalf("compactText() = %q, want %q", got, "铁甲战士 ...")
	}
}

func TestCompactTextHandlesSmallLimits(t *testing.T) {
	got := compactText("防御", 2)
	if got != "防御" {
		t.Fatalf("compactText() = %q, want %q", got, "防御")
	}
}
