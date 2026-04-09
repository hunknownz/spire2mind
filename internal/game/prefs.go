package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FastModeStatus struct {
	Path     string `json:"path"`
	Previous string `json:"previous"`
	Current  string `json:"current"`
	Desired  string `json:"desired,omitempty"`
	Changed  bool   `json:"changed"`
}

func NormalizeFastMode(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "normal", "fast", "instant":
		return value
	default:
		return value
	}
}

func ReadFastMode(path string) (*FastModeStatus, error) {
	status := &FastModeStatus{Path: filepath.Clean(path)}
	if strings.TrimSpace(path) == "" {
		return status, fmt.Errorf("prefs path is empty")
	}

	doc, err := loadPrefsDocument(path)
	if err != nil {
		return status, err
	}

	status.Current = NormalizeFastMode(stringValue(doc["fast_mode"]))
	status.Previous = status.Current
	return status, nil
}

func EnsureFastMode(path string, desired string) (*FastModeStatus, error) {
	status := &FastModeStatus{
		Path:    filepath.Clean(path),
		Desired: NormalizeFastMode(desired),
	}
	if strings.TrimSpace(path) == "" {
		return status, fmt.Errorf("prefs path is empty")
	}

	doc, err := loadPrefsDocument(path)
	if err != nil {
		return status, err
	}

	current := NormalizeFastMode(stringValue(doc["fast_mode"]))
	status.Previous = current
	status.Current = current
	if status.Desired == "" || status.Desired == current {
		return status, nil
	}

	doc["fast_mode"] = status.Desired
	if err := writePrefsDocument(path, doc); err != nil {
		return status, err
	}

	status.Current = status.Desired
	status.Changed = true
	return status, nil
}

func loadPrefsDocument(path string) (map[string]interface{}, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read prefs %s: %w", filepath.Clean(path), err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode prefs %s: %w", filepath.Clean(path), err)
	}

	return doc, nil
}

func writePrefsDocument(path string, doc map[string]interface{}) error {
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode prefs %s: %w", filepath.Clean(path), err)
	}
	encoded = append(encoded, '\n')

	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("write prefs %s: %w", filepath.Clean(path), err)
	}

	return nil
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return text
}
