# Shared Lightsail defaults for the stock quote bot.
# Sourced by deploy-stock-bot-lightsail.sh and stock-bot-lightsail-logs.sh.
# LIGHTSAIL_REGION wins over AWS_DEFAULT_REGION so a leftover CLI region
# (this account's secret is us-west-2) cannot silently deploy elsewhere.
SERVICE_NAME="${LIGHTSAIL_SERVICE_NAME:-stock-bot}"
CONTAINER_NAME="${LIGHTSAIL_CONTAINER_NAME:-stock-bot}"
REGION="${LIGHTSAIL_REGION:-us-east-2}"
POWER="${LIGHTSAIL_POWER:-nano}"
SCALE="${LIGHTSAIL_SCALE:-1}"
IMAGE_LABEL="${LIGHTSAIL_IMAGE_LABEL:-stock-bot}"
LOCAL_IMAGE="${LOCAL_IMAGE:-stock-bot:latest}"
