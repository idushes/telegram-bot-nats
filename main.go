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
	webhookBase := os.Getenv("WEBHOOK_BASE_URL")
	port := env("PORT", "8080")
	secret := os.Getenv("WEBHOOK_SECRET")

	if webhookBase == "" {
		log.Fatal("WEBHOOK_BASE_URL is required")
	}
	// Remove trailing slash
	webhookBase = strings.TrimRight(webhookBase, "/")

	// Discover bots from BOT_* env vars
	bots := discoverBots()
	if len(bots) == 0 {
		log.Fatal("No bots configured. Set BOT_<NAME>=<token> environment variables.")
	}

	log.Printf("Starting telegram-bot-nats (webhook) | NATS: %s | Bots: %d", natsURL, len(bots))

	// NATS connect
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("NATS connect: %v", err)
	}
	defer nc.Close()

	// Build bot lookup map for webhook handler
	botMap := make(map[string]Bot, len(bots))

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

		// Set webhook at Telegram
		webhookURL := fmt.Sprintf("%s/webhook/%s", webhookBase, b.Name)
		if err := setWebhook(b, webhookURL, secret); err != nil {
			log.Fatalf("[%s] setWebhook: %v", b.Name, err)
		}
		log.Printf("[%s] webhook set: %s", b.Name, webhookURL)

		botMap[b.Name] = b
	}

	// HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/", makeWebhookHandler(nc, botMap, secret))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start HTTP server
	go func() {
		log.Printf("HTTP server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server: %v", err)
		}
	}()

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")

	// Delete webhooks so polling works again in dev
	for _, b := range bots {
		if err := deleteWebhook(b); err != nil {
			log.Printf("[%s] deleteWebhook: %v", b.Name, err)
		} else {
			log.Printf("[%s] webhook deleted", b.Name)
		}
	}

	// Shutdown HTTP server
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	srv.Shutdown(shutCtx)

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

// makeWebhookHandler returns an HTTP handler for Telegram webhook updates.
func makeWebhookHandler(nc *nats.Conn, botMap map[string]Bot, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract bot name from path: /webhook/<name>
		name := strings.TrimPrefix(r.URL.Path, "/webhook/")
		if name == "" {
			http.Error(w, "missing bot name", http.StatusBadRequest)
			return
		}

		bot, ok := botMap[name]
		if !ok {
			http.Error(w, "unknown bot", http.StatusNotFound)
			return
		}

		// Verify secret token
		if secret != "" {
			token := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
			if token != secret {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}

		var upd Update
		if err := json.Unmarshal(body, &upd); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		publishUpdate(nc, bot, upd)

		w.WriteHeader(http.StatusOK)
	}
}

// setWebhook registers the webhook URL at Telegram for a bot.
func setWebhook(bot Bot, webhookURL, secret string) error {
	params := map[string]interface{}{
		"url":             webhookURL,
		"allowed_updates": []string{"message", "edited_message", "callback_query", "inline_query"},
	}
	if secret != "" {
		params["secret_token"] = secret
	}

	payload, _ := json.Marshal(params)
	_, err := callTelegramAPI(bot.Token, "setWebhook", payload)
	return err
}

// deleteWebhook removes the webhook at Telegram for a bot.
func deleteWebhook(bot Bot) error {
	_, err := callTelegramAPI(bot.Token, "deleteWebhook", []byte(`{}`))
	return err
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
