# Claude Bumper Lanes Test Suite

# Run all tests (default)
default: test

# Run all test suites
test: test-unit test-integration

# Run all unit tests
test-unit:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Running unit tests..."
    for test in tests/unit/test-*.sh; do
        echo ""
        "$test" || exit 1
    done

# Run integration tests
test-integration:
    @echo "Running integration tests..."
    ./tests/integration/validate-hook-contracts.sh

# Run specific unit test
test-unit-specific TEST:
    ./tests/unit/{{TEST}}.sh

# Run manual threshold trip test
test-manual-trip:
    ./tests/manual/trip-threshold.sh

# Generate test file (450 lines to trip threshold)
generate:
    @echo "Generating test file..."
    @for i in {1..450}; do \
        echo "// Generated line $$i - This is test code to trip the bumper lanes threshold" >> generated-code.txt; \
    done
    @echo "✓ Generated generated-code.txt (450 lines)"

# Clean generated files
clean:
    rm -f generated-code.txt test-output.txt different-file.txt
    @echo "✓ Cleaned generated files"

# List available unit tests
list:
    @echo "Available unit tests:"
    @ls tests/unit/test-*.sh | xargs -n1 basename | sed 's/\.sh$$//'

# Run smoke test (verify libs load)
smoke:
    ./tests/unit/test-lib-simple.sh
