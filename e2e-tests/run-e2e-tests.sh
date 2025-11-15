#!/bin/bash
set -ux

# Container engine: defaults to docker, can be overridden:
# e.g.: ENGINE=podman TF_PLUGIN_PLATFORM=linux_arm64 ./run-e2e-tests.sh
ENGINE="${ENGINE:-docker}"
COMPOSE="$ENGINE compose"
TF_PLUGIN_PLATFORM="${TF_PLUGIN_PLATFORM:-linux_amd64}"

# Start in background
TF_PLUGIN_PLATFORM="$TF_PLUGIN_PLATFORM" \
  $COMPOSE up -d --build

# Start streaming logs in the background (non-blocking)
$COMPOSE logs -f &
LOGS_PID=$!

# Wait for recursor-health to be healthy
$ENGINE inspect --format='{{.State.Health.Status}}' "recursor-health"
while true; do
    STATUS="$($ENGINE inspect --format='{{.State.Health.Status}}' "recursor-health" 2>/dev/null || echo "starting")"
    if [ "$STATUS" = "healthy" ]; then
        echo "Service 'recursor-health' is healthy!"
        break
    fi
    if [ "$STATUS" = "unhealthy" ]; then
        echo "Service 'recursor-health' is UNHEALTHY!"
        exit 1
    fi
    sleep 10
done

# Wait for terraform to finish
TF_STATUS="$($ENGINE wait terraform)"

# Stop log streaming
kill "$LOGS_PID" || true

$COMPOSE down --remove-orphans

# Remove the volume (ignore error if it doesn't exist)
$ENGINE volume rm bootstrap_pdns_data || true

exit "$TF_STATUS"
