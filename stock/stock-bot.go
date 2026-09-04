package main

import (
	"bytes"
	"context"
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
	"github.com/yattoni/discord-bots/stock/openrouter"
	"github.com/yattoni/discord-bots/stock/quoteimg"
	"github.com/yattoni/discord-bots/stock/yahoo"
)

func main() {
	preview := flag.String("preview", "", "fetch a ticker and write a quote PNG, then exit")
	out := flag.String("out", "quote.png", "output path used with -preview")
	ask := flag.String("ask", "", "send a prompt to MiniMax via OpenRouter and print the reply")
	flag.Parse()

	client := yahoo.NewClient()

	if *preview != "" {
		if err := writePreview(client, *preview, *out); err != nil {
			log.Fatal(err)
		}
		return
	}

	router := newOpenRouterFromEnv()
	if *ask != "" {
		if err := writeAsk(router, *ask); err != nil {
			log.Fatal(err)
		}
		return
	}

	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_BOT_TOKEN is required (or pass -preview TICKER / -ask PROMPT)")
	}

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	bot := &stockBot{
		session:    session,
		yahoo:      client,
		openrouter: router,
		channelID:  strings.TrimSpace(os.Getenv("DISCORD_CHANNEL_ID")),
	}
	session.AddHandler(bot.onMessage)
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentMessageContent | discordgo.IntentsDirectMessages

	if err := session.Open(); err != nil {
		log.Fatal("Error opening Discord websocket: ", err)
	}
	defer session.Close()

	if router != nil {
		log.Println("Stock bot is listening for $TICKER messages, @mention help, and @mention chat")
	} else {
		log.Println("Stock bot is listening for $TICKER messages and @mention help (set OPENROUTER_API_KEY for @mention chat)")
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop
	log.Println("Shutting down")
}

func newOpenRouterFromEnv() *openrouter.Client {
	key := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if key == "" {
		return nil
	}
	return openrouter.NewClient(key)
}

func writeAsk(client *openrouter.Client, prompt string) error {
	if client == nil {
		return fmt.Errorf("OPENROUTER_API_KEY is required with -ask")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	reply, err := client.Complete(ctx, mentionSystemPrompt, prompt)
	if err != nil {
		return err
	}
	fmt.Println(reply)
	return nil
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
	session    *discordgo.Session
	yahoo      *yahoo.Client
	openrouter *openrouter.Client
	channelID  string

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

	if MentionsBot(botID, m.Content, userIDs(m.Mentions)) {
		go b.replyToMention(s, m)
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

func (b *stockBot) replyToMention(s *discordgo.Session, m *discordgo.MessageCreate) {
	logProcessed(m, "mention")
	if b.openrouter == nil {
		replyText(s, m, mentionErrorReply(openrouter.ErrUnauthorized))
		return
	}

	prompt := MentionPrompt(m.Content)
	if prompt == "" {
		prompt = "The user mentioned you without additional text."
	}

	if err := s.ChannelTyping(m.ChannelID); err != nil {
		log.Printf("failed to send typing indicator: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	reply, err := b.openrouter.Complete(ctx, mentionSystemPrompt, prompt)
	if err != nil {
		log.Printf("mention chat failed: %v", err)
		replyText(s, m, mentionErrorReply(err))
		return
	}
	replyText(s, m, clipDiscordMessage(reply))
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
