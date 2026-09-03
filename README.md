# discord-bots

Discord bots that fetch and post earthquakes, gas prices, and stock quotes.

## Stock quote bot

A websocket Discord bot that watches for messages that are only a ticker, like `$NOW`, then replies with a Yahoo Finance quote card:

- current price
- today's dollar and percent change (green up / red down)
- 1-minute chart covering premarket, regular hours, and after hours (green above the previous close, red below)

Tickers are 1–5 letters, with an optional share class (`$BRK.B`). The rest of the message must be empty aside from whitespace.

Mention the bot with `help` (for example `@stock-bot help`) to get a short explanation of how it works. Messages the bot processes are written to its logs.

This bot has to stay connected to Discord's gateway (websocket). Incoming messages cannot be read with the webhook-only gas and quake bots.

### Discord setup

1. Create an application at https://discord.com/developers/applications
2. Add a bot user and copy its token
3. Enable the **Message Content Intent**
4. Invite the bot with permission to view channels, send messages, attach files, and read message history:

   `https://discord.com/oauth2/authorize?client_id=YOUR_APP_ID&permissions=117760&scope=bot`

### Run

```sh
export DISCORD_BOT_TOKEN=your-bot-token
# optional: only respond in one channel
# export DISCORD_CHANNEL_ID=1234567890
go run ./stock
```

Preview a quote image without Discord:

```sh
go run ./stock -preview NOW -out now.png
```

### Docker

The image is a static linux/amd64 binary (what Lightsail expects). It needs `DISCORD_BOT_TOKEN`, and optionally `DISCORD_CHANNEL_ID` to limit replies to one channel.

```sh
docker build --platform linux/amd64 -t stock-bot .
docker run -d --name stock-bot --restart unless-stopped \
  -e DISCORD_BOT_TOKEN \
  -e DISCORD_CHANNEL_ID \
  stock-bot
```

Or with Compose (reads `DISCORD_BOT_TOKEN` from the environment or a local `.env` file):

```sh
docker compose up -d --build
```

Preview a card inside the container without Discord:

```sh
docker run --rm -v "$PWD:/out" -u "$(id -u):$(id -g)" \
  stock-bot -preview NOW -out /out/now.png
```

### AWS Lightsail

**Container service** (managed, no VM to SSH into):

1. Create a service: `aws lightsail create-container-service --service-name stock-bot --power nano --scale 1`
2. Build and push: `docker build --platform linux/amd64 -t stock-bot . && aws lightsail push-container-image --service-name stock-bot --label stock-bot --image stock-bot:latest`
3. Deploy from the Lightsail console: add a container named `stock-bot`, image `:stock-bot.latest`, and environment variables `DISCORD_BOT_TOKEN` (required) and `DISCORD_CHANNEL_ID` (optional). Do **not** open a public endpoint — this bot only talks to Discord over the websocket gateway.

**Instance** (a small Linux VM with Docker):

```sh
sudo apt-get update && sudo apt-get install -y docker.io docker-compose-v2
sudo usermod -aG docker $USER   # log out and back in after this
docker compose up -d --build
```

Keep the token in `/etc/environment` or a `.env` file next to `docker-compose.yml`. The compose file restarts the container unless you stop it.
