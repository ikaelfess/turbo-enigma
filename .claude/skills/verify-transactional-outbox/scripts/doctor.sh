#!/usr/bin/env bash
# Read-only: is this instance ours and worth driving? Exit 0 = yes.
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
fail=0; chk() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi; }
chk "project $COMPOSE_PROJECT_NAME has an api container" 'dc ps -q api | grep -q .'
chk "api published on host port $VERIFY_API_PORT is ours" "dc port api 3000 | grep -q ':$VERIFY_API_PORT\$'"
chk "GET /health returns OK" "[ \"\$(curl -fsS $API/health)\" = OK ]"
chk "migrations applied (outbox_events.claimed_until exists)" "[ \"\$(psql_q \"select count(*) from information_schema.columns where table_name='outbox_events' and column_name='claimed_until'\")\" = 1 ]"
chk "migrations applied (consumed_events exists)" "[ \"\$(psql_q \"select to_regclass('consumed_events') is not null\")\" = t ]"
chk "topic order.created has 3 partitions" "dc exec -T kafka kafka-topics --bootstrap-server kafka:9092 --describe --topic order.created | grep -q PartitionCount:.3"
echo "publisher: $(dc ps -q outbox_event_publisher | grep -q . && echo running || echo stopped)"
echo "consumer: $(dc ps -q outbox_event_consumer | grep -q . && echo running || echo stopped)"
exit $fail
