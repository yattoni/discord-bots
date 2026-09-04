# discord-bots

Discord bots that fetch and post earthquakes, gas prices, and stock quotes.

## Stock quote bot

A websocket Discord bot that watches for messages that are only a ticker, like `$NOW`, then replies with a Yahoo Finance quote card:

- current price
- today's dollar and percent change (green up / red down)
- 1-minute chart covering premarket, regular hours, and after hours for stocks, or 24h for crypto (green above the previous close, red below)

Tickers are 1–5 letters, with an optional share class (`$BRK.B`) or crypto pair (`$BTC-USD`). The rest of the message must be empty aside from whitespace.

`$BTC` is treated as Bitcoin spot (`BTC-USD`). Other crypto pairs use Yahoo's hyphenated names, like `$ETH-USD`.

Mention the bot with `help` (for example `@stock-bot help`) to get a short explanation of how it works. Mention it with any other question and it replies using MiniMax M3 (`minimax/minimax-m3:free`) through OpenRouter. Messages the bot processes are written to its logs.

Unknown or delisted tickers get a not-found reply. If Yahoo is down or the quote card can't be attached, the bot says so instead of pretending the ticker is missing, and falls back to a text quote when it already has the numbers.

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
# optional: required for @mention chat replies
# export OPENROUTER_API_KEY=your-openrouter-key
go run ./stock
```

Preview a quote image without Discord:

```sh
go run ./stock -preview NOW -out now.png
```

Ask MiniMax without Discord (needs `OPENROUTER_API_KEY`):

```sh
go run ./stock -ask "What does a P/E ratio mean?"
```

### Docker

The image is a static linux/amd64 binary (what Lightsail expects). It needs `DISCORD_BOT_TOKEN`, and optionally `DISCORD_CHANNEL_ID` to limit replies to one channel and `OPENROUTER_API_KEY` for @mention chat.

```sh
docker build --platform linux/amd64 -t stock-bot .
docker run -d --name stock-bot --restart unless-stopped \
  -e DISCORD_BOT_TOKEN \
  -e DISCORD_CHANNEL_ID \
  -e OPENROUTER_API_KEY \
  stock-bot
```

Or with Compose (reads `DISCORD_BOT_TOKEN` and optional `OPENROUTER_API_KEY` from the environment or a local `.env` file):

```sh
docker compose up -d --build
```

Preview a card inside the container without Discord:

```sh
docker run --rm -v "$PWD:/out" -u "$(id -u):$(id -g)" \
  stock-bot -preview NOW -out /out/now.png
```

### AWS Lightsail

**Container service** (managed, no VM to SSH into). Install AWS CLI v2, Docker, and [lightsailctl](https://docs.aws.amazon.com/lightsail/latest/userguide/amazon-lightsail-install-software.html), then:

```sh
export DISCORD_BOT_TOKEN=your-bot-token
# optional: only respond in one channel
# export DISCORD_CHANNEL_ID=1234567890
# optional: required for @mention chat replies
# export OPENROUTER_API_KEY=your-openrouter-key
./scripts/deploy-stock-bot-lightsail.sh
```

The script creates a `nano` service named `stock-bot` in `us-east-2` if needed, builds `linux/amd64`, pushes the image, and deploys `DISCORD_BOT_TOKEN` (plus optional `DISCORD_CHANNEL_ID` and `OPENROUTER_API_KEY`). It does **not** open a public endpoint — this bot only talks to Discord over the websocket gateway.

Fetch container logs:

```sh
./scripts/stock-bot-lightsail-logs.sh
./scripts/stock-bot-lightsail-logs.sh --filter ERROR
./scripts/stock-bot-lightsail-logs.sh --follow
```

Override defaults with `LIGHTSAIL_REGION`, `LIGHTSAIL_SERVICE_NAME`, `LIGHTSAIL_POWER`, or `LIGHTSAIL_SCALE`. Region is `us-east-2` unless you set `LIGHTSAIL_REGION`; `AWS_DEFAULT_REGION` is ignored so a leftover CLI region cannot send the deploy elsewhere.

**Instance** (a small Linux VM with Docker):

```sh
sudo apt-get update && sudo apt-get install -y docker.io docker-compose-v2
sudo usermod -aG docker $USER   # log out and back in after this
docker compose up -d --build
```

Keep the token in `/etc/environment` or a `.env` file next to `docker-compose.yml`. The compose file restarts the container unless you stop it.
