#!/usr/bin/env bash
# Tear down only the outbox-verify project, including its volumes. Evidence in $VERIFY_OUT is kept.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
dc down -v --remove-orphans
