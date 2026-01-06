#!/bin/bash
# Post-deployment hook script
# This script runs after deployment completes

set -e

TARGET="${1:-unknown}"
REMOTE_USER="${2}"
REMOTE_HOST="${3}"
REMOTE_PATH="${4}"

echo "Running post-deployment actions for target: $TARGET"

if [ -z "$REMOTE_USER" ] || [ -z "$REMOTE_HOST" ]; then
    echo "Error: Remote connection details not provided"
    exit 1
fi

# Send reload signal to server (production mode)
echo "Sending reload signal to server..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "pkill -SIGHUP tqserver || true"

# Wait for server to reload
sleep 2

# Perform health check
echo "Checking server health..."
HEALTH_URL="http://${REMOTE_HOST}:8080/health"
HEALTH_CHECK=$(ssh "${REMOTE_USER}@${REMOTE_HOST}" "curl -s -o /dev/null -w '%{http_code}' ${HEALTH_URL}" || echo "000")

if [ "$HEALTH_CHECK" = "200" ]; then
    echo "✓ Health check passed"
else
    echo "⚠ Health check failed (HTTP $HEALTH_CHECK)"
    exit 1
fi

# Optional: Send notification
# curl -X POST https://hooks.slack.com/... -d "{'text':'Deployment to $TARGET completed'}"

echo "✓ Post-deployment actions completed"
exit 0
