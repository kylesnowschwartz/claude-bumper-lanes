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
