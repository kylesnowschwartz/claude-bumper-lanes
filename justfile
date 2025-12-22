# Claude Bumper Lanes Test Suite (Bats Framework)

# Run all tests (default)
default: test

# Run all test suites
test: test-unit test-integration

# Run all unit tests with Bats
test-unit:
    tests/bats/bin/bats tests/unit/*.bats

# Run integration tests with Bats
test-integration:
    tests/bats/bin/bats tests/integration/*.bats

# Run tests with TAP output (for CI)
test-tap:
    tests/bats/bin/bats --tap tests/**/*.bats

# Run specific test file (without .bats extension)
test-file FILE:
    tests/bats/bin/bats tests/unit/{{FILE}}.bats

# Initialize git submodules (run after clone)
setup-tests:
    git submodule update --init --recursive

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

# List available test files and tags
list:
    @echo "Available test files:"
    @echo ""
    @echo "Unit tests:"
    @ls tests/unit/*.bats | xargs -n1 basename | sed 's/\.bats$$//'
    @echo ""
    @echo "Integration tests:"
    @ls tests/integration/*.bats | xargs -n1 basename | sed 's/\.bats$$//'
    @echo ""
    @echo "Common tags: threshold, weighting, incremental, formatting, hooks"

# Show Bats version
version:
    tests/bats/bin/bats --version

# ─────────────────────────────────────────────────────────────
# Go Tools (diff-viz)
# ─────────────────────────────────────────────────────────────

# Build all Go tools
build: build-diff-viz

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
    rm -f bumper-lanes-plugin/tools/diff-viz/git-diff-tree
    @echo "Cleaned Go binaries"

# Run Go tests
test-go:
    cd bumper-lanes-plugin/tools/diff-viz && go test ./...

# Format Go code
fmt-go:
    cd bumper-lanes-plugin/tools/diff-viz && go fmt ./...

# Check Go code (vet + build)
check-go:
    cd bumper-lanes-plugin/tools/diff-viz && go vet ./... && go build ./...
