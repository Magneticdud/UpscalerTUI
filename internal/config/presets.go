package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SavedPreset is the JSON schema for a user-saved preset (compatible with Python app).
type SavedPreset struct {
	ImageType string `json:"image_type"`
	Scale     string `json:"scale"`
	TTA       bool   `json:"tta"`
}

// presetsPath returns the path to presets.json next to the executable.
func presetsPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "presets.json"), nil
}

// LoadPresets reads saved presets. Returns empty map on any error.
func LoadPresets() map[string]SavedPreset {
	path, err := presetsPath()
	if err != nil {
		return map[string]SavedPreset{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]SavedPreset{}
	}
	var result map[string]SavedPreset
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]SavedPreset{}
	}
	return result
}

// SavePreset saves a preset by name. Overwrites if already exists.
func SavePreset(name string, preset SavedPreset) error {
	path, err := presetsPath()
	if err != nil {
		return err
	}
	presets := LoadPresets()
	presets[name] = preset
	data, err := json.MarshalIndent(presets, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
