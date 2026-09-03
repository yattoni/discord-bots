# discord-bots

Discord bots that fetch and post earthquakes, gas prices, and stock quotes.

## Stock quote bot

A websocket Discord bot that watches for messages that are only a ticker, like `$NOW`, then replies with a Yahoo Finance quote card:

- current price
- today's dollar and percent change (green up / red down)
- 1-minute chart covering premarket, regular hours, and after hours (green above the previous close, red below)

Tickers are 1–5 letters, with an optional share class (`$BRK.B`). The rest of the message must be empty aside from whitespace.

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
