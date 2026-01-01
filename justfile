# Claude Bumper Lanes - Go Implementation
# Part of bumper-lanes-dev workspace (../go.work)

# Run all tests (default)
default: test

# ─────────────────────────────────────────────────────────────
# Workspace commands (passthrough to ../justfile)
# ─────────────────────────────────────────────────────────────

# Sync to latest diff-viz tag
sync-deps:
    just -f ../justfile sync-deps

# Verify build without workspace (test real deps)
verify-upstream:
    just -f ../justfile verify-upstream

# Show status of both repos
workspace-status:
    just -f ../justfile status

# ─────────────────────────────────────────────────────────────
# Local commands
# ─────────────────────────────────────────────────────────────

# Run all Go tests
test: test-go

# Run Go tests for bumper-lanes
test-go:
    cd bumper-lanes-plugin/tools/bumper-lanes && go test ./...

# Run tests with verbose output
test-verbose:
    cd bumper-lanes-plugin/tools/bumper-lanes && go test -v ./...

# Run manual threshold trip test
test-manual-trip:
    ./tests/manual/trip-threshold.sh

# Generate test file (450 lines to trip threshold)
generate:
    @echo "Generating test file..."
    @for i in {1..450}; do \
        echo "// Generated line $$i - This is test code to trip the bumper lanes threshold" >> generated-code.txt; \
    done
    @echo "Generated generated-code.txt (450 lines)"

# Clean generated files
clean:
    rm -f generated-code.txt test-output.txt different-file.txt
    @echo "Cleaned generated files"

# Show Go version
version:
    go version

# ─────────────────────────────────────────────────────────────
# Go Build
# ─────────────────────────────────────────────────────────────

# Build bumper-lanes
build: build-bumper-lanes

# Build the bumper-lanes tool
build-bumper-lanes:
    cd bumper-lanes-plugin/tools/bumper-lanes && go build -o ../../bin/bumper-lanes ./cmd/bumper-lanes
    @echo "Built: bumper-lanes-plugin/bin/bumper-lanes"

# ─────────────────────────────────────────────────────────────
# diff-viz (external dependency)
# ─────────────────────────────────────────────────────────────

# Install diff-viz CLI globally from GitHub
install-diff-viz:
    go install github.com/kylesnowschwartz/diff-viz/v2/cmd/git-diff-tree@latest
    @echo "Installed: git-diff-tree (via go install)"

# Copy git-diff-tree to plugin bin directory (for bundled distribution)
bundle-diff-viz:
    @mkdir -p bumper-lanes-plugin/bin
    cp $(shell which git-diff-tree) bumper-lanes-plugin/bin/git-diff-tree
    @echo "Bundled: bumper-lanes-plugin/bin/git-diff-tree"

# Clean Go build artifacts
clean-go:
    rm -f bumper-lanes-plugin/bin/bumper-lanes
    rm -f bumper-lanes-plugin/bin/git-diff-tree
    @echo "Cleaned Go binaries"

# Format Go code
fmt-go:
    cd bumper-lanes-plugin/tools/bumper-lanes && go fmt ./...

# Check Go code (vet + build)
check-go:
    cd bumper-lanes-plugin/tools/bumper-lanes && go vet ./... && go build ./...

# ─────────────────────────────────────────────────────────────
# Version management
# ─────────────────────────────────────────────────────────────

# Show current version
show-version:
    @jq -r '.version' bumper-lanes-plugin/.claude-plugin/plugin.json

# Bump plugin version (single source of truth)
bump version:
    @jq '.version = "{{version}}"' bumper-lanes-plugin/.claude-plugin/plugin.json > /tmp/plugin.json && mv /tmp/plugin.json bumper-lanes-plugin/.claude-plugin/plugin.json
    @echo "Version is now: {{version}}"
