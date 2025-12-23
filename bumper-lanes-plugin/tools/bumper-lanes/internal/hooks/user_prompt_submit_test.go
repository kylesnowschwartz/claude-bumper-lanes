package hooks

import (
	"testing"
)

func TestCommandPatterns(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		wantCmd string
	}{
		{
			name:    "reset command",
			prompt:  "/claude-bumper-lanes:bumper-reset",
			wantCmd: "reset",
		},
		{
			name:    "pause command",
			prompt:  "/claude-bumper-lanes:bumper-pause",
			wantCmd: "pause",
		},
		{
			name:    "resume command",
			prompt:  "/claude-bumper-lanes:bumper-resume",
			wantCmd: "resume",
		},
		{
			name:    "view command",
			prompt:  "/claude-bumper-lanes:bumper-view tree",
			wantCmd: "view",
		},
		{
			name:    "config command",
			prompt:  "/claude-bumper-lanes:bumper-config",
			wantCmd: "config",
		},
		{
			name:    "config set command",
			prompt:  "/claude-bumper-lanes:bumper-config set 500",
			wantCmd: "config",
		},
		{
			name:    "no command - regular message",
			prompt:  "just a regular message",
			wantCmd: "",
		},
		{
			name:    "command embedded in text",
			prompt:  "please run /claude-bumper-lanes:bumper-reset for me",
			wantCmd: "reset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := detectCommand(tt.prompt)
			if cmd != tt.wantCmd {
				t.Errorf("detectCommand(%q) = %q, want %q", tt.prompt, cmd, tt.wantCmd)
			}
		})
	}
}

func TestViewModeExtraction(t *testing.T) {
	tests := []struct {
		prompt   string
		wantMode string
	}{
		{"/claude-bumper-lanes:bumper-view tree", "tree"},
		{"/claude-bumper-lanes:bumper-view icicle", "icicle"},
		{"/claude-bumper-lanes:bumper-view collapsed", "collapsed"},
		{"/claude-bumper-lanes:bumper-view", ""}, // no mode specified
		{"set view to /claude-bumper-lanes:bumper-view brackets please", "brackets"},
	}

	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			matches := viewModePattern.FindStringSubmatch(tt.prompt)
			mode := ""
			if len(matches) > 1 {
				mode = matches[1]
			}
			if mode != tt.wantMode {
				t.Errorf("viewModePattern on %q = %q, want %q", tt.prompt, mode, tt.wantMode)
			}
		})
	}
}

func TestConfigArgsExtraction(t *testing.T) {
	tests := []struct {
		prompt     string
		wantAction string
		wantValue  string
	}{
		{"/claude-bumper-lanes:bumper-config show", "show", ""},
		{"/claude-bumper-lanes:bumper-config set 500", "set", "500"},
		{"/claude-bumper-lanes:bumper-config personal 300", "personal", "300"},
		{"/claude-bumper-lanes:bumper-config", "", ""}, // no args
	}

	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			matches := configArgsPattern.FindStringSubmatch(tt.prompt)
			action := ""
			value := ""
			if len(matches) > 1 {
				action = matches[1]
			}
			if len(matches) > 2 {
				value = matches[2]
			}
			if action != tt.wantAction {
				t.Errorf("configArgsPattern action on %q = %q, want %q", tt.prompt, action, tt.wantAction)
			}
			if value != tt.wantValue {
				t.Errorf("configArgsPattern value on %q = %q, want %q", tt.prompt, value, tt.wantValue)
			}
		})
	}
}

// detectCommand is a test helper that mirrors the logic in UserPromptSubmitFromStdin
func detectCommand(prompt string) string {
	switch {
	case contains(prompt, cmdReset):
		return "reset"
	case contains(prompt, cmdPause):
		return "pause"
	case contains(prompt, cmdResume):
		return "resume"
	case contains(prompt, cmdView):
		return "view"
	case contains(prompt, cmdConfig):
		return "config"
	default:
		return ""
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
