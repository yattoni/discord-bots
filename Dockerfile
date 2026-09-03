# Stock quote Discord bot. Build for Lightsail (linux/amd64):
#   docker build --platform linux/amd64 -t stock-bot .
# Run:
#   docker run --rm -e DISCORD_BOT_TOKEN -e DISCORD_CHANNEL_ID stock-bot
FROM golang:1.27-bookworm AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY stock ./stock

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/stock-bot ./stock

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/stock-bot /stock-bot

USER nonroot:nonroot
ENTRYPOINT ["/stock-bot"]
