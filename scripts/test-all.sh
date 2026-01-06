#!/bin/bash
# Comprehensive test suite for TQServer
# Run all tests before deployment

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== TQServer Test Suite ===${NC}\n"

# Track failures
FAILED=0

# Helper function
run_test() {
    local test_name="$1"
    local test_cmd="$2"
    
    echo -e "${YELLOW}Testing: ${test_name}${NC}"
    if eval "$test_cmd"; then
        echo -e "${GREEN}✓ ${test_name} passed${NC}\n"
    else
        echo -e "${RED}✗ ${test_name} failed${NC}\n"
        FAILED=$((FAILED + 1))
    fi
}

# 1. Unit Tests
run_test "Unit Tests" "go test ./pkg/..."

# 2. Build Tests
run_test "Development Build" "./scripts/build-dev.sh > /dev/null 2>&1"
run_test "Production Build" "./scripts/build-prod.sh > /dev/null 2>&1"

# 3. Binary Verification
run_test "Server Binary Exists" "test -f server/bin/tqserver"
run_test "Worker Binary Exists" "test -n \"\$(ls workers/index/bin/tqworker_* 2>/dev/null || ls workers/index/bin/index 2>/dev/null)\""

# 4. Configuration Validation
run_test "Server Config Valid" "python3 -c 'import yaml; yaml.safe_load(open(\"config/server.yaml\"))' 2>/dev/null"
run_test "Deployment Config Valid" "python3 -c 'import yaml; yaml.safe_load(open(\"config/deployment.yaml\"))' 2>/dev/null"

# 5. Script Syntax Checks
run_test "Deploy Script Syntax" "bash -n scripts/deploy.sh"
run_test "Pre-deploy Hook Syntax" "bash -n scripts/hooks/pre-deploy.sh"
run_test "Post-deploy Hook Syntax" "bash -n scripts/hooks/post-deploy.sh"
run_test "Build Dev Script Syntax" "bash -n scripts/build-dev.sh"
run_test "Build Prod Script Syntax" "bash -n scripts/build-prod.sh"

# 6. Go Code Quality
run_test "Go Vet" "go vet ./..."
run_test "Go Fmt Check" "test -z \"\$(gofmt -l .)\""

# 7. Module Verification
run_test "Go Mod Tidy" "go mod tidy && git diff --exit-code go.mod go.sum"

# Summary
echo -e "\n${GREEN}=== Test Summary ===${NC}"
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed! ✓${NC}"
    exit 0
else
    echo -e "${RED}$FAILED test(s) failed ✗${NC}"
    exit 1
fi
