# Claude Bumper Lanes - Go Implementation

# Run all tests (default)
default: test

# Run all Go tests
test: test-go

# Run Go tests for all packages
test-go:
    cd bumper-lanes-plugin/tools/diff-viz && go test ./...
    cd bumper-lanes-plugin/tools/bumper-lanes && go test ./...

# Run tests with verbose output
test-verbose:
    cd bumper-lanes-plugin/tools/diff-viz && go test -v ./...
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

# Build all Go tools
build: build-diff-viz build-bumper-lanes

# Build the bumper-lanes tool
build-bumper-lanes:
    cd bumper-lanes-plugin/tools/bumper-lanes && go build -o ../../bin/bumper-lanes ./cmd/bumper-lanes
    @echo "Built: bumper-lanes-plugin/bin/bumper-lanes"

# Build the diff-viz tool
build-diff-viz:
    cd bumper-lanes-plugin/tools/diff-viz && go build -o ../../bin/git-diff-tree-go ./cmd/git-diff-tree
    @echo "Built: bumper-lanes-plugin/bin/git-diff-tree-go"

# Run diff-viz directly (builds first)
diff-viz *ARGS:
    @just build-diff-viz
    ./bumper-lanes-plugin/bin/git-diff-tree-go {{ARGS}}

# Install diff-viz to ~/.local/bin (symlink)
install-diff-viz:
    @just build-diff-viz
    @mkdir -p ~/.local/bin
    ln -sf "$(pwd)/bumper-lanes-plugin/bin/git-diff-tree-go" ~/.local/bin/git-diff-tree
    @echo "Installed: ~/.local/bin/git-diff-tree"

# Uninstall diff-viz
uninstall-diff-viz:
    rm -f ~/.local/bin/git-diff-tree
    @echo "Removed: ~/.local/bin/git-diff-tree"

# Clean Go build artifacts
clean-go:
    rm -f bumper-lanes-plugin/bin/git-diff-tree-go
    rm -f bumper-lanes-plugin/bin/bumper-lanes
    rm -f bumper-lanes-plugin/tools/diff-viz/git-diff-tree
    @echo "Cleaned Go binaries"

# Format Go code
fmt-go:
    cd bumper-lanes-plugin/tools/diff-viz && go fmt ./...
    cd bumper-lanes-plugin/tools/bumper-lanes && go fmt ./...

# Check Go code (vet + build)
check-go:
    cd bumper-lanes-plugin/tools/diff-viz && go vet ./... && go build ./...
    cd bumper-lanes-plugin/tools/bumper-lanes && go vet ./... && go build ./...
