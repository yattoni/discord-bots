#!/usr/bin/env bash
# Deploy the stock quote Discord bot to an AWS Lightsail container service.
# Required: AWS credentials, Docker daemon, AWS CLI v2, lightsailctl, DISCORD_BOT_TOKEN.
# Optional: DISCORD_CHANNEL_ID, OPENROUTER_API_KEY, LIGHTSAIL_REGION (default us-east-2),
#           LIGHTSAIL_SERVICE_NAME, LIGHTSAIL_POWER, LIGHTSAIL_SCALE.
# The shared env file clears AWS_PAGER so CLI v2 does not fail without `less`.
# docker info is checked up front; see the README if dockerd is not running.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lightsail-stock-bot.env.sh
source "$ROOT/scripts/lightsail-stock-bot.env.sh"

if [[ -z "${DISCORD_BOT_TOKEN:-}" ]]; then
  echo "DISCORD_BOT_TOKEN is required" >&2
  exit 1
fi

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

need aws
need docker
need jq
if ! command -v lightsailctl >/dev/null 2>&1; then
  echo "missing lightsailctl (https://docs.aws.amazon.com/lightsail/latest/userguide/amazon-lightsail-install-software.html)" >&2
  exit 1
fi

# `docker` on PATH is not enough: these VMs often have no systemd, so
# `systemctl start docker` / `service docker start` do nothing.
if ! docker info >/dev/null 2>&1; then
  echo "docker daemon is not running (or this user cannot reach /var/run/docker.sock)" >&2
  echo "On hosts without systemd, start dockerd yourself:" >&2
  echo "  sudo dockerd --host=unix:///var/run/docker.sock --iptables=false >/tmp/dockerd.log 2>&1 &" >&2
  echo "  sudo chmod 666 /var/run/docker.sock   # if you are not in the docker group" >&2
  echo "If overlay storage fails in a nested VM, restart with --storage-driver=vfs --data-root=/var/lib/docker-vfs" >&2
  exit 1
fi

wait_until_pushable() {
  local i state
  for i in $(seq 1 60); do
    state="$(aws lightsail get-container-services \
      --service-name "$SERVICE_NAME" \
      --region "$REGION" \
      --query 'containerServices[0].state' \
      --output text)"
    echo "$(date -u +%H:%M:%S) service state=$state"
    case "$state" in
      READY|RUNNING) return 0 ;;
      DISABLED|DELETING)
        echo "service entered unexpected state $state" >&2
        return 1
        ;;
    esac
    sleep 15
  done
  echo "timed out waiting for service to become READY or RUNNING" >&2
  return 1
}

existing="$(aws lightsail get-container-services \
  --service-name "$SERVICE_NAME" \
  --region "$REGION" \
  --query 'containerServices[0].containerServiceName' \
  --output text 2>/dev/null || true)"

if [[ "$existing" != "$SERVICE_NAME" ]]; then
  echo "creating Lightsail container service $SERVICE_NAME ($POWER x$SCALE) in $REGION"
  aws lightsail create-container-service \
    --service-name "$SERVICE_NAME" \
    --power "$POWER" \
    --scale "$SCALE" \
    --region "$REGION" \
    --query 'containerService.{name:containerServiceName,state:state}' \
    --output json
fi

wait_until_pushable

echo "building $LOCAL_IMAGE for linux/amd64"
docker build --platform linux/amd64 -t "$LOCAL_IMAGE" "$ROOT"

echo "pushing $LOCAL_IMAGE to Lightsail service $SERVICE_NAME"
push_out="$(aws lightsail push-container-image \
  --region "$REGION" \
  --service-name "$SERVICE_NAME" \
  --label "$IMAGE_LABEL" \
  --image "$LOCAL_IMAGE")"
echo "$push_out"

image="$(printf '%s\n' "$push_out" | sed -n 's/.*Refer to this image as "\([^"]*\)".*/\1/p')"
if [[ -z "$image" ]]; then
  image="$(aws lightsail get-container-images \
    --service-name "$SERVICE_NAME" \
    --region "$REGION" \
    --query 'containerImages[0].image' \
    --output text)"
fi
echo "using image $image"

deploy_json="$(mktemp)"
trap 'rm -f "$deploy_json"' EXIT
chmod 600 "$deploy_json"

jq -n \
  --arg service "$SERVICE_NAME" \
  --arg image "$image" \
  --arg token "$DISCORD_BOT_TOKEN" \
  --arg channel "${DISCORD_CHANNEL_ID:-}" \
  --arg openrouter "${OPENROUTER_API_KEY:-}" \
  '
  def env:
    {DISCORD_BOT_TOKEN: $token}
    + (if $channel != "" then {DISCORD_CHANNEL_ID: $channel} else {} end)
    + (if $openrouter != "" then {OPENROUTER_API_KEY: $openrouter} else {} end);
  {
    serviceName: $service,
    containers: {
      "stock-bot": {
        image: $image,
        environment: env
      }
    }
  }' > "$deploy_json"

echo "creating deployment (no public endpoint)"
aws lightsail create-container-service-deployment \
  --region "$REGION" \
  --cli-input-json "file://$deploy_json" \
  --query 'containerService.{name:containerServiceName,state:state,next:nextDeployment.state}' \
  --output json

for i in $(seq 1 40); do
  info="$(aws lightsail get-container-services \
    --service-name "$SERVICE_NAME" \
    --region "$REGION" \
    --query 'containerServices[0].{state:state,dep:currentDeployment.state}' \
    --output json)"
  echo "$(date -u +%H:%M:%S) $info"
  state="$(echo "$info" | jq -r .state)"
  dep="$(echo "$info" | jq -r .dep)"
  if [[ "$state" == "RUNNING" && "$dep" == "ACTIVE" ]]; then
    echo "stock-bot is RUNNING on Lightsail ($SERVICE_NAME in $REGION)"
    exit 0
  fi
  if [[ "$state" == "DISABLED" ]]; then
    echo "service disabled" >&2
    exit 1
  fi
  sleep 15
done

echo "timed out waiting for RUNNING/ACTIVE" >&2
exit 1
