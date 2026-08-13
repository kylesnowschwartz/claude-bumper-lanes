package enforce

import (
	"testing"
)

func TestMatchTripwirePath(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{".github/workflows/**", ".github/workflows/ci.yml", true},
		{".github/workflows/**", ".github/workflows/deploy/prod.yml", true},
		{".github/workflows/**", ".github/dependabot.yml", false},
		{"go.mod", "go.mod", true},
		{"go.mod", "tools/api/go.mod", true}, // basename match for slashless patterns
		{"go.mod", "go.sum", false},
		{"**/hooks.json", "plugin/hooks/hooks.json", true},
		{"**/hooks.json", "hooks.json", true},
		{"**/migrations/**", "app/db/migrations/0001_init.sql", true},
		{"**/migrations/**", "app/db/schema.sql", false},
		{".claude/settings*.json", ".claude/settings.json", true},
		{".claude/settings*.json", ".claude/settings.local.json", true},
		{".claude/settings*.json", ".claude/skills/x.json", false},
		{"db/migrate/**", "db/migrate/20260811_add.rb", true},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+"~"+tt.path, func(t *testing.T) {
			if got := matchTripwirePath(tt.pattern, tt.path); got != tt.want {
				t.Errorf("matchTripwirePath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}
