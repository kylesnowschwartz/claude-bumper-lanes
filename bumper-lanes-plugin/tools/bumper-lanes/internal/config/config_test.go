package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidMode(t *testing.T) {
	tests := []struct {
		mode  string
		valid bool
	}{
		{"tree", true},
		{"collapsed", true},
		{"smart", true},
		{"topn", true},
		{"icicle", true},
		{"brackets", true},
		{"invalid", false},
		{"", false},
		{"TREE", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			if got := isValidMode(tt.mode); got != tt.valid {
				t.Errorf("isValidMode(%q) = %v, want %v", tt.mode, got, tt.valid)
			}
		})
	}
}

func TestConfigStruct(t *testing.T) {
	// Test JSON marshaling/unmarshaling
	cfg := Config{
		Threshold:       500,
		DefaultViewMode: "icicle",
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if loaded.Threshold != 500 {
		t.Errorf("Threshold = %d, want 500", loaded.Threshold)
	}
	if loaded.DefaultViewMode != "icicle" {
		t.Errorf("DefaultViewMode = %q, want %q", loaded.DefaultViewMode, "icicle")
	}
}

func TestLoadConfigFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	configJSON := `{"threshold": 300, "default_view_mode": "collapsed"}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := loadConfigFile(configPath)
	if err != nil {
		t.Fatalf("loadConfigFile failed: %v", err)
	}

	if cfg.Threshold != 300 {
		t.Errorf("Threshold = %d, want 300", cfg.Threshold)
	}
	if cfg.DefaultViewMode != "collapsed" {
		t.Errorf("DefaultViewMode = %q, want %q", cfg.DefaultViewMode, "collapsed")
	}
}

func TestLoadConfigFile_Missing(t *testing.T) {
	_, err := loadConfigFile("/nonexistent/path/config.json")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestLoadConfigFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad-config.json")

	if err := os.WriteFile(configPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := loadConfigFile(configPath)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestDefaultValues(t *testing.T) {
	if DefaultThreshold != 400 {
		t.Errorf("DefaultThreshold = %d, want 400", DefaultThreshold)
	}
	if DefaultViewMode != "tree" {
		t.Errorf("DefaultViewMode = %q, want %q", DefaultViewMode, "tree")
	}
}
