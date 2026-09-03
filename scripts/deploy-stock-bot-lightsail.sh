#!/usr/bin/env bash
# Deploy the stock quote Discord bot to an AWS Lightsail container service.
# Required: AWS credentials, Docker, AWS CLI v2, lightsailctl, DISCORD_BOT_TOKEN.
# Optional: DISCORD_CHANNEL_ID, LIGHTSAIL_REGION (default us-east-2),
#           LIGHTSAIL_SERVICE_NAME, LIGHTSAIL_POWER, LIGHTSAIL_SCALE.
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

if [[ -n "${DISCORD_CHANNEL_ID:-}" ]]; then
  jq -n \
    --arg service "$SERVICE_NAME" \
    --arg image "$image" \
    --arg token "$DISCORD_BOT_TOKEN" \
    --arg channel "$DISCORD_CHANNEL_ID" \
    '{
      serviceName: $service,
      containers: {
        "stock-bot": {
          image: $image,
          environment: {
            DISCORD_BOT_TOKEN: $token,
            DISCORD_CHANNEL_ID: $channel
          }
        }
      }
    }' > "$deploy_json"
else
  jq -n \
    --arg service "$SERVICE_NAME" \
    --arg image "$image" \
    --arg token "$DISCORD_BOT_TOKEN" \
    '{
      serviceName: $service,
      containers: {
        "stock-bot": {
          image: $image,
          environment: {
            DISCORD_BOT_TOKEN: $token
          }
        }
      }
    }' > "$deploy_json"
fi

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
