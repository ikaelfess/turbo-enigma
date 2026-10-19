#!/usr/bin/env bash
# Start the outbox event consumer. It joins group outbox-event-consumer and reads
# topic order.created from the start, so it also picks up records published earlier.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
dc up --build -d outbox_event_consumer
