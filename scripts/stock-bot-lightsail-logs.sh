#!/usr/bin/env bash
# Print Lightsail container logs for the stock quote Discord bot.
# Optional: LIGHTSAIL_REGION (default us-east-2), LIGHTSAIL_SERVICE_NAME,
#           LIGHTSAIL_CONTAINER_NAME, --filter PATTERN, --follow
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lightsail-stock-bot.env.sh
source "$ROOT/lightsail-stock-bot.env.sh"

FILTER=""
FOLLOW=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --filter)
      FILTER="${2:-}"
      if [[ -z "$FILTER" ]]; then
        echo "usage: $0 [--filter PATTERN] [--follow]" >&2
        exit 1
      fi
      shift 2
      ;;
    --follow|-f)
      FOLLOW=1
      shift
      ;;
    -h|--help)
      echo "usage: $0 [--filter PATTERN] [--follow]"
      echo "  LIGHTSAIL_REGION          default us-east-2"
      echo "  LIGHTSAIL_SERVICE_NAME    default stock-bot"
      echo "  LIGHTSAIL_CONTAINER_NAME  default stock-bot"
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      echo "usage: $0 [--filter PATTERN] [--follow]" >&2
      exit 1
      ;;
  esac
done

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

need aws
need jq

fetch_logs() {
  local args=(
    lightsail get-container-log
    --service-name "$SERVICE_NAME"
    --container-name "$CONTAINER_NAME"
    --region "$REGION"
    --output json
  )
  if [[ -n "$FILTER" ]]; then
    args+=(--filter-pattern "$FILTER")
  fi
  aws "${args[@]}"
}

print_events() {
  jq -r '.logEvents[]? | "\(.createdAt)\t\(.message)"'
}

if [[ "$FOLLOW" -eq 0 ]]; then
  echo "logs for $SERVICE_NAME/$CONTAINER_NAME in $REGION"
  fetch_logs | print_events
  exit 0
fi

echo "following logs for $SERVICE_NAME/$CONTAINER_NAME in $REGION (Ctrl-C to stop)"
seen=""
while true; do
  events="$(fetch_logs | print_events || true)"
  if [[ -n "$events" ]]; then
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      if [[ "$seen" != *$'\n'"$line"$'\n'* ]]; then
        printf '%s\n' "$line"
        seen+="$line"$'\n'
      fi
    done <<< "$events"
    # keep the seen set from growing without bound
    if [[ "${#seen}" -gt 200000 ]]; then
      seen="$(printf '%s' "$seen" | tail -c 100000)"
    fi
  fi
  sleep 10
done
