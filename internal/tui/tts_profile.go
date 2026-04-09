package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ttsProfileSnapshot struct {
	Name          string  `json:"name"`
	Provider      string  `json:"provider"`
	Voice         string  `json:"voice"`
	Speed         float64 `json:"speed"`
	StreamerStyle string  `json:"streamerStyle"`
}

func (m *Model) refreshTTSProfile() {
	if m == nil || strings.TrimSpace(m.repoRoot) == "" {
		return
	}

	profile := readTTSProfileSnapshot(m.repoRoot)
	m.ttsProfileName = compactValue(profile.Name)
	m.ttsProfileProvider = compactValue(profile.Provider)
	m.ttsProfileVoice = compactValue(profile.Voice)
	if profile.Speed > 0 {
		m.ttsProfileSpeed = strconv.FormatFloat(profile.Speed, 'f', 2, 64)
	}
	if strings.TrimSpace(profile.StreamerStyle) != "" {
		m.streamerStyle = profile.StreamerStyle
	}
}

func readTTSProfileSnapshot(repoRoot string) ttsProfileSnapshot {
	path := filepath.Join(repoRoot, "scratch", "tts", "provider-profile.json")
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return ttsProfileSnapshot{
			Name:          "melotts-default",
			Provider:      "melotts",
			Voice:         "female",
			Speed:         1.00,
			StreamerStyle: "bright-cute",
		}
	}

	var snapshot ttsProfileSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return ttsProfileSnapshot{
			Name:          "melotts-default",
			Provider:      "melotts",
			Voice:         "female",
			Speed:         1.00,
			StreamerStyle: "bright-cute",
		}
	}

	if strings.TrimSpace(snapshot.Name) == "" {
		snapshot.Name = "melotts-default"
	}
	if strings.TrimSpace(snapshot.Provider) == "" {
		snapshot.Provider = "melotts"
	}
	if strings.TrimSpace(snapshot.Voice) == "" {
		snapshot.Voice = "female"
	}
	if snapshot.Speed <= 0 {
		snapshot.Speed = 1.00
	}
	if strings.TrimSpace(snapshot.StreamerStyle) == "" {
		snapshot.StreamerStyle = "bright-cute"
	}

	return snapshot
}
