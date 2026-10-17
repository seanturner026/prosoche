package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
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

// Sender delivers one titled message. Implementations handle their own size limits.
type Sender interface {
	Send(ctx context.Context, title, body string) error
}

type discordSender struct {
	baseURL   string
	channelID string
	botToken  string
}

// Send posts the message to the channel, spreading long passages over several posts.
func (d discordSender) Send(ctx context.Context, title, body string) error {
	for _, chunk := range split("**"+title+"**\n\n"+body, discordMaxLen) {
		if err := d.post(ctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (d discordSender) post(ctx context.Context, content string) error {
	body, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		d.baseURL+"/channels/"+d.channelID+"/messages",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+d.botToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord api returned status %d: %s", resp.StatusCode, b)
	}
	return nil
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

func message(p passage.Passage) (title, body string) {
	return fmt.Sprintf("%s: %s %s", p.Author, p.Work, p.Ref()), p.Text
}

func run(ctx context.Context, ps []passage.Passage, now time.Time, s Sender) error {
	p := passage.ForDate(ps, now)
	title, body := message(p)
	if err := s.Send(ctx, title, body); err != nil {
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
		title, body := message(passage.ForDate(ps, day))
		fmt.Printf("%s\n\n%s\n", title, body)
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
