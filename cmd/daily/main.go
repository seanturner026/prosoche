package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/seanturner026/prosoche/internal/passage"
)

const (
	discordAPIBase = "https://discord.com/api/v10"
	discordMaxLen  = 2000
)

// Message is one day's post. Reply, when set, is threaded under it.
type Message struct {
	Title, Body, Reply string
}

// ErrReply means the message was delivered but its reply was not.
var ErrReply = errors.New("reply not delivered")

// Sender delivers a Message. Implementations handle their own size limits.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

type discordSender struct {
	baseURL   string
	channelID string
	botToken  string
}

// Send posts the message to the channel, spreading long passages over several posts, then
// threads the reply under the last post so it sits where reading ends.
func (d discordSender) Send(ctx context.Context, m Message) error {
	var lastID string
	for _, chunk := range split("**"+m.Title+"**\n\n"+m.Body, discordMaxLen) {
		id, err := d.post(ctx, "/channels/"+d.channelID+"/messages", map[string]string{"content": chunk})
		if err != nil {
			return err
		}
		lastID = id
	}
	if m.Reply == "" {
		return nil
	}

	threadID, err := d.post(ctx, "/channels/"+d.channelID+"/messages/"+lastID+"/threads", map[string]string{"name": m.Title})
	if err != nil {
		return fmt.Errorf("%w: create thread: %w", ErrReply, err)
	}
	for _, chunk := range split(m.Reply, discordMaxLen) {
		if _, err := d.post(ctx, "/channels/"+threadID+"/messages", map[string]string{"content": chunk}); err != nil {
			return fmt.Errorf("%w: %w", ErrReply, err)
		}
	}
	return nil
}

// post sends payload to a Discord API path and returns the created object's ID.
func (d discordSender) post(ctx context.Context, path string, payload map[string]string) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bot "+d.botToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("discord api %s returned status %d: %s", path, resp.StatusCode, b)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &created); err != nil {
		return "", fmt.Errorf("decode discord response: %w", err)
	}
	return created.ID, nil
}

// split breaks s into chunks of at most limit bytes, preferring paragraph, then sentence,
// then word boundaries.
func split(s string, limit int) []string {
	var chunks []string
	for len(s) > limit {
		cut := strings.LastIndex(s[:limit], "\n\n")
		if cut <= 0 {
			cut = strings.LastIndex(s[:limit], ". ") + 1
		}
		if cut <= 0 {
			cut = strings.LastIndex(s[:limit], " ")
		}
		if cut <= 0 {
			cut = limit
		}
		chunks = append(chunks, strings.TrimSpace(s[:cut]))
		s = strings.TrimSpace(s[cut:])
	}
	return append(chunks, s)
}

// getDiscordSender posts to DISCORD_CHANNEL_ID using the bot_token key of the JSON blob at
// SSM_PARAMETER (captain-hook's).
func getDiscordSender(ctx context.Context) (discordSender, error) {
	channelID := os.Getenv("DISCORD_CHANNEL_ID")
	if channelID == "" {
		return discordSender{}, fmt.Errorf("DISCORD_CHANNEL_ID must be set")
	}
	token, err := getBotToken(ctx)
	if err != nil {
		return discordSender{}, err
	}
	return discordSender{baseURL: discordAPIBase, channelID: channelID, botToken: token}, nil
}

func getBotToken(ctx context.Context) (string, error) {
	paramName := os.Getenv("SSM_PARAMETER")
	if paramName == "" {
		return "", fmt.Errorf("SSM_PARAMETER must be set")
	}
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("load aws config: %w", err)
	}
	withDecryption := true
	out, err := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &paramName,
		WithDecryption: &withDecryption,
	})
	if err != nil {
		return "", fmt.Errorf("ssm get parameter %s: %w", paramName, err)
	}
	if out.Parameter == nil || out.Parameter.Value == nil {
		return "", fmt.Errorf("ssm parameter %s has no value", paramName)
	}
	var blob map[string]string
	if err := json.Unmarshal([]byte(*out.Parameter.Value), &blob); err != nil {
		return "", fmt.Errorf("unmarshal ssm parameter %s: %w", paramName, err)
	}
	token, ok := blob["bot_token"]
	if !ok {
		return "", fmt.Errorf("key %q not found in ssm parameter %s", "bot_token", paramName)
	}
	return token, nil
}

func message(p passage.Passage) Message {
	return Message{
		Title: fmt.Sprintf("%s: %s %s", p.Author, p.Work, p.Ref()),
		Body:  p.Text,
		Reply: p.Interpretation,
	}
}

func run(ctx context.Context, ps []passage.Passage, now time.Time, s Sender) error {
	p := passage.ForDate(ps, now)
	err := s.Send(ctx, message(p))
	if errors.Is(err, ErrReply) {
		// Returning an error would make Lambda retry and post the passage again.
		slog.Error("interpretation not sent", "id", p.ID, "error", err)
		return nil
	}
	if err != nil {
		return fmt.Errorf("send %s: %w", p.ID, err)
	}
	slog.Info("passage sent", "id", p.ID, "words", p.WordCount)
	return nil
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	preview := flag.String("preview", "", "print the message for this date (YYYY-MM-DD) instead of sending")
	flag.Parse()

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		slog.Error("load timezone", "error", err)
		os.Exit(1)
	}
	ps, err := passage.Load()
	if err != nil {
		slog.Error("load passages", "error", err)
		os.Exit(1)
	}

	if *preview != "" {
		day, err := time.ParseInLocation(time.DateOnly, *preview, loc)
		if err != nil {
			slog.Error("parse preview date", "error", err)
			os.Exit(1)
		}
		m := message(passage.ForDate(ps, day))
		fmt.Printf("%s\n\n%s\n", m.Title, m.Body)
		if m.Reply != "" {
			fmt.Printf("\n--- thread ---\n\n%s\n", m.Reply)
		}
		return
	}

	handler := func(ctx context.Context) error {
		s, err := getDiscordSender(ctx)
		if err != nil {
			return fmt.Errorf("get discord config: %w", err)
		}
		return run(ctx, ps, time.Now().In(loc), s)
	}

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(handler)
	} else if err := handler(context.Background()); err != nil {
		slog.Error("run failed", "error", err)
		os.Exit(1)
	}
}
