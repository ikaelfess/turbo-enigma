#!/usr/bin/env bash
# Start an isolated stack (project outbox-verify): Postgres, migrations, API, Kafka, topic.
# The publisher is NOT started here; run scripts/start-publisher.sh when a feature needs it.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
if lsof -nP -iTCP:"$VERIFY_API_PORT" -sTCP:LISTEN >/dev/null 2>&1 && ! dc ps -q api | grep -q .; then
  echo "port $VERIFY_API_PORT is held by something else; set VERIFY_API_PORT" >&2; exit 1
fi
(cd "$ROOT" && make env >/dev/null)
dc up --build -d api kafka kafka_topics
for i in $(seq 1 120); do
  if curl -fsS "$API/health" >/dev/null 2>&1 \
     && [ "$(dc ps -q kafka | xargs docker inspect -f '{{.State.Health.Status}}')" = healthy ] \
     && [ "$(dc ps -a -q kafka_topics | xargs docker inspect -f '{{.State.ExitCode}}')" = 0 ]; then
    echo "ready: $API"; exit 0
  fi
  sleep 2
done
echo "not ready after 240s" >&2; dc ps >&2; exit 1
