# Sourced by the other scripts and by bash or zsh driving shells (run from inside the repo).
ROOT="$(git rev-parse --show-toplevel)"
SKILL="$ROOT/.claude/skills/verify-transactional-outbox"
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-outbox-verify}"
export VERIFY_API_PORT="${VERIFY_API_PORT:-13000}"
export VERIFY_UI_PORT="${VERIFY_UI_PORT:-18080}"
export VERIFY_OUT="${VERIFY_OUT:-$ROOT/.verify/artifacts}"
API="http://127.0.0.1:$VERIFY_API_PORT"
dc() { (cd "$ROOT" && docker compose -f compose.yml -f "$SKILL/compose.verify.yml" "$@"); }
psql_q() { dc exec -T postgres psql -U postgres -d transactional_outbox_development -At -c "$1"; }
