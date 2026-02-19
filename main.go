package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
)

// Bot — one Telegram bot instance.
type Bot struct {
	Name  string // lowercase name from ENV (e.g. "my_bot")
	Token string // Telegram Bot API token
}

// Update — minimal Telegram Update struct for routing.
type Update struct {
	UpdateID      int              `json:"update_id"`
	Message       *json.RawMessage `json:"message,omitempty"`
	EditedMessage *json.RawMessage `json:"edited_message,omitempty"`
	CallbackQuery *json.RawMessage `json:"callback_query,omitempty"`
	InlineQuery   *json.RawMessage `json:"inline_query,omitempty"`
}

// RawRequest — payload for out.raw subject.
type RawRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func main() {
	_ = godotenv.Load()

	natsURL := env("NATS_URL", "nats://localhost:4222")

	// Discover bots from BOT_* env vars
	bots := discoverBots()
	if len(bots) == 0 {
		log.Fatal("No bots configured. Set BOT_<NAME>=<token> environment variables.")
	}

	log.Printf("Starting telegram-bot-nats | NATS: %s | Bots: %d", natsURL, len(bots))

	// NATS connect
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("NATS connect: %v", err)
	}
	defer nc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start each bot
	for _, bot := range bots {
		b := bot
		log.Printf("[%s] starting bot", b.Name)

		// Subscribe to outgoing subjects: telegram.<name>.out.>
		subject := fmt.Sprintf("telegram.%s.out.>", b.Name)
		_, err := nc.Subscribe(subject, func(msg *nats.Msg) {
			handleOutgoing(b, msg)
		})
		if err != nil {
			log.Fatalf("[%s] subscribe %s: %v", b.Name, subject, err)
		}
		log.Printf("[%s] subscribed to %s", b.Name, subject)

		// Start long-polling goroutine
		go pollUpdates(ctx, nc, b)
	}

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
	cancel()
	nc.Drain()
	log.Println("Done.")
}

// discoverBots scans environment for BOT_* variables.
func discoverBots() []Bot {
	var bots []Bot
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		if !strings.HasPrefix(key, "BOT_") || val == "" {
			continue
		}
		name := strings.ToLower(strings.TrimPrefix(key, "BOT_"))
		bots = append(bots, Bot{Name: name, Token: val})
	}
	return bots
}

// pollUpdates performs Telegram long-polling and publishes updates to NATS.
func pollUpdates(ctx context.Context, nc *nats.Conn, bot Bot) {
	offset := 0
	client := &http.Client{Timeout: 35 * time.Second}
	baseURL := fmt.Sprintf("https://api.telegram.org/bot%s", bot.Token)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		url := fmt.Sprintf("%s/getUpdates?offset=%d&timeout=30", baseURL, offset)
		resp, err := client.Get(url)
		if err != nil {
			log.Printf("[%s] poll error: %v", bot.Name, err)
			time.Sleep(3 * time.Second)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("[%s] read body: %v", bot.Name, err)
			time.Sleep(3 * time.Second)
			continue
		}

		var result struct {
			OK     bool     `json:"ok"`
			Result []Update `json:"result"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			log.Printf("[%s] unmarshal: %v", bot.Name, err)
			time.Sleep(3 * time.Second)
			continue
		}

		if !result.OK {
			log.Printf("[%s] API returned ok=false: %s", bot.Name, string(body))
			time.Sleep(5 * time.Second)
			continue
		}

		for _, upd := range result.Result {
			offset = upd.UpdateID + 1
			publishUpdate(nc, bot, upd)
		}
	}
}

// publishUpdate publishes a Telegram update to NATS subjects.
func publishUpdate(nc *nats.Conn, bot Bot, upd Update) {
	data, err := json.Marshal(upd)
	if err != nil {
		log.Printf("[%s] marshal update: %v", bot.Name, err)
		return
	}

	prefix := fmt.Sprintf("telegram.%s.in", bot.Name)

	// Always publish full update
	if err := nc.Publish(prefix+".update", data); err != nil {
		log.Printf("[%s] publish update: %v", bot.Name, err)
	}

	// Route by type
	switch {
	case upd.Message != nil:
		nc.Publish(prefix+".message", []byte(*upd.Message))
	case upd.EditedMessage != nil:
		nc.Publish(prefix+".edited", []byte(*upd.EditedMessage))
	case upd.CallbackQuery != nil:
		nc.Publish(prefix+".callback", []byte(*upd.CallbackQuery))
	case upd.InlineQuery != nil:
		nc.Publish(prefix+".inline", []byte(*upd.InlineQuery))
	}

	log.Printf("[%s] ← update %d", bot.Name, upd.UpdateID)
}

// handleOutgoing handles NATS messages on telegram.<name>.out.* subjects.
func handleOutgoing(bot Bot, msg *nats.Msg) {
	// Extract method from subject: telegram.<name>.out.<method>
	parts := strings.Split(msg.Subject, ".")
	if len(parts) < 4 {
		log.Printf("[%s] bad out subject: %s", bot.Name, msg.Subject)
		respondError(msg, "invalid subject format")
		return
	}
	method := parts[len(parts)-1]

	var apiMethod string
	var payload []byte

	if method == "raw" {
		// Parse raw request to get actual method
		var raw RawRequest
		if err := json.Unmarshal(msg.Data, &raw); err != nil {
			log.Printf("[%s] bad raw request: %v", bot.Name, err)
			respondError(msg, fmt.Sprintf("bad raw request: %v", err))
			return
		}
		apiMethod = raw.Method
		payload = raw.Params
	} else {
		apiMethod = method
		payload = msg.Data
	}

	// Call Telegram Bot API
	result, err := callTelegramAPI(bot.Token, apiMethod, payload)
	if err != nil {
		log.Printf("[%s] API %s error: %v", bot.Name, apiMethod, err)
		respondError(msg, fmt.Sprintf("API error: %v", err))
		return
	}

	log.Printf("[%s] → %s OK", bot.Name, apiMethod)

	// Reply if request/reply pattern
	if msg.Reply != "" {
		msg.Respond(result)
	}
}

// callTelegramAPI calls a Telegram Bot API method with JSON payload.
func callTelegramAPI(token, method string, payload []byte) ([]byte, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, method)

	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return body, nil
}

// respondError sends an error response if reply subject is set.
func respondError(msg *nats.Msg, errMsg string) {
	if msg.Reply == "" {
		return
	}
	resp := map[string]string{"error": errMsg}
	data, _ := json.Marshal(resp)
	msg.Respond(data)
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
