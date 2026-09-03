package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/yattoni/discord-bots/stock/quoteimg"
	"github.com/yattoni/discord-bots/stock/yahoo"
)

func main() {
	preview := flag.String("preview", "", "fetch a ticker and write a quote PNG, then exit")
	out := flag.String("out", "quote.png", "output path used with -preview")
	flag.Parse()

	client := yahoo.NewClient()

	if *preview != "" {
		if err := writePreview(client, *preview, *out); err != nil {
			log.Fatal(err)
		}
		return
	}

	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_BOT_TOKEN is required (or pass -preview TICKER to render a quote image)")
	}

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	bot := &stockBot{
		session:   session,
		yahoo:     client,
		channelID: strings.TrimSpace(os.Getenv("DISCORD_CHANNEL_ID")),
	}
	session.AddHandler(bot.onMessage)
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentMessageContent | discordgo.IntentsDirectMessages

	if err := session.Open(); err != nil {
		log.Fatal("Error opening Discord websocket: ", err)
	}
	defer session.Close()

	log.Println("Stock bot is listening for $TICKER messages and @mention help")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop
	log.Println("Shutting down")
}

func writePreview(client *yahoo.Client, ticker, path string) error {
	quote, err := client.FetchQuote(ResolveTicker(ticker))
	if err != nil {
		return err
	}
	png, err := quoteimg.RenderPNG(quote)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, png, 0644); err != nil {
		return err
	}
	log.Printf("Wrote %s quote card to %s", quote.Symbol, path)
	return nil
}

type stockBot struct {
	session   *discordgo.Session
	yahoo     *yahoo.Client
	channelID string

	mu    sync.Mutex
	cache map[string]cachedQuote
}

type cachedQuote struct {
	quote   *yahoo.Quote
	png     []byte
	fetched time.Time
}

func (b *stockBot) onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot {
		return
	}
	if s.State != nil && s.State.User != nil && m.Author.ID == s.State.User.ID {
		return
	}
	if b.channelID != "" && m.ChannelID != b.channelID {
		return
	}

	botID := ""
	if s.State != nil && s.State.User != nil {
		botID = s.State.User.ID
	}

	if IsHelpMention(botID, m.Content, userIDs(m.Mentions)) {
		logProcessed(m, "help")
		if _, err := s.ChannelMessageSendReply(m.ChannelID, helpMessage, m.Reference()); err != nil {
			log.Printf("failed to send help reply: %v", err)
		}
		return
	}

	ticker, ok := ParseTicker(m.Content)
	if !ok {
		return
	}

	logProcessed(m, "$"+ticker)
	quote, png, err := b.lookup(ticker)
	if err != nil {
		log.Printf("quote failed for %s: %v", ticker, err)
		replyText(s, m, quoteErrorReply(ticker, err))
		return
	}

	_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files: []*discordgo.File{{
			Name:        fmt.Sprintf("%s.png", quote.Symbol),
			ContentType: "image/png",
			Reader:      bytes.NewReader(png),
		}},
		Reference: m.Reference(),
	})
	if err != nil {
		log.Printf("failed to send quote image for %s: %v", ticker, err)
		replyText(s, m, quoteImageFallback(quote))
	}
}

func replyText(s *discordgo.Session, m *discordgo.MessageCreate, content string) {
	if _, err := s.ChannelMessageSendReply(m.ChannelID, content, m.Reference()); err != nil {
		log.Printf("failed to send reply: %v", err)
	}
}

func userIDs(users []*discordgo.User) []string {
	ids := make([]string, 0, len(users))
	for _, u := range users {
		if u != nil && u.ID != "" {
			ids = append(ids, u.ID)
		}
	}
	return ids
}

func logProcessed(m *discordgo.MessageCreate, kind string) {
	author := "unknown"
	if m.Author != nil {
		author = m.Author.Username
	}
	log.Printf("processed %s message from %s in channel %s: %q", kind, author, m.ChannelID, m.Content)
}

func (b *stockBot) lookup(ticker string) (*yahoo.Quote, []byte, error) {
	b.mu.Lock()
	if b.cache == nil {
		b.cache = map[string]cachedQuote{}
	}
	if cached, ok := b.cache[ticker]; ok && time.Since(cached.fetched) < 20*time.Second {
		b.mu.Unlock()
		return cached.quote, cached.png, nil
	}
	b.mu.Unlock()

	quote, err := b.yahoo.FetchQuote(ticker)
	if err != nil {
		return nil, nil, err
	}
	png, err := quoteimg.RenderPNG(quote)
	if err != nil {
		return nil, nil, err
	}

	b.mu.Lock()
	b.cache[ticker] = cachedQuote{quote: quote, png: png, fetched: time.Now()}
	b.mu.Unlock()
	return quote, png, nil
}
