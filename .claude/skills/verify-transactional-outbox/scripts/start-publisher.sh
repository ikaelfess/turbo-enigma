#!/usr/bin/env bash
# Start the outbox publisher. River runs it once at startup, then every minute,
# so rows created BEFORE this call publish within seconds.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
dc up --build -d outbox_event_publisher
