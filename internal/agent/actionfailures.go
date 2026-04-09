package agentruntime

import "spire2mind/internal/game"

type actionFailureMemory struct {
	digest  string
	blocked map[string]struct{}
}

func newActionFailureMemory() *actionFailureMemory {
	return &actionFailureMemory{
		blocked: make(map[string]struct{}),
	}
}

func (m *actionFailureMemory) ResetForDigest(digest string) {
	if m == nil {
		return
	}
	if m.digest == digest {
		return
	}
	m.digest = digest
	clear(m.blocked)
}

func (m *actionFailureMemory) Record(digest string, request game.ActionRequest) {
	if m == nil || digest == "" {
		return
	}
	m.ResetForDigest(digest)
	m.blocked[actionFailureKey(request)] = struct{}{}
}

func (m *actionFailureMemory) Allows(digest string, request game.ActionRequest) bool {
	if m == nil {
		return true
	}
	if m.digest != digest {
		return true
	}
	_, blocked := m.blocked[actionFailureKey(request)]
	return !blocked
}

func actionFailureKey(request game.ActionRequest) string {
	return formatActionDebug(request)
}
