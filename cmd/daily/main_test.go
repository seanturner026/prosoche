package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/seanturner026/prosoche/internal/passage"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  []string
	}{
		{name: "fits in one chunk", in: "short", limit: 10, want: []string{"short"}},
		{name: "prefers paragraph break", in: "one two.\n\nthree four.", limit: 15, want: []string{"one two.", "three four."}},
		{name: "falls back to sentence", in: "One. Two three four", limit: 12, want: []string{"One.", "Two three", "four"}},
		{name: "falls back to word", in: "aaa bbb ccc", limit: 8, want: []string{"aaa bbb", "ccc"}},
		{name: "hard cut without spaces", in: "abcdefgh", limit: 3, want: []string{"abc", "def", "gh"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := split(tt.in, tt.limit)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("split(%q, %d) = %q, want %q", tt.in, tt.limit, got, tt.want)
			}
		})
	}
}

func TestSplitAllPassages(t *testing.T) {
	ps, err := passage.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		chunks := split(p.Text, discordMaxLen)
		for _, c := range chunks {
			if len(c) > discordMaxLen {
				t.Errorf("%s: chunk of %d bytes exceeds %d", p.ID, len(c), discordMaxLen)
			}
		}
		if got := len(strings.Fields(strings.Join(chunks, " "))); got != p.WordCount {
			t.Errorf("%s: chunks hold %d words, want %d", p.ID, got, p.WordCount)
		}
	}
}

func TestDiscordSend(t *testing.T) {
	var contents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/channels/chan1/messages" || r.Header.Get("Authorization") != "Bot tok" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		b, _ := io.ReadAll(r.Body)
		var msg struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(b, &msg); err != nil {
			t.Errorf("decode body: %v", err)
		}
		contents = append(contents, msg.Content)
	}))
	defer srv.Close()

	d := discordSender{baseURL: srv.URL, channelID: "chan1", botToken: "tok"}
	body := strings.Repeat("word ", 300) + "\n\n" + strings.Repeat("more ", 300)
	if err := d.Send(context.Background(), "Epictetus: Enchiridion 1", body); err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if len(contents) != 2 {
		t.Fatalf("got %d posts, want 2", len(contents))
	}
	if !strings.HasPrefix(contents[0], "**Epictetus: Enchiridion 1**\n\nword") {
		t.Errorf("first post = %q, want bold title then passage", contents[0][:40])
	}

	bad := discordSender{baseURL: srv.URL, channelID: "chan1", botToken: "wrong"}
	if err := bad.Send(context.Background(), "t", "b"); err == nil {
		t.Error("Send() with rejected token: want error")
	}
}

type mockSender struct {
	title, body string
	err         error
}

func (m *mockSender) Send(_ context.Context, title, body string) error {
	m.title, m.body = title, body
	return m.err
}

func TestRun(t *testing.T) {
	ps := []passage.Passage{
		{ID: "meditations-4.3", Author: "Marcus Aurelius", Work: "Meditations", Book: 4, Section: 3, Text: "Men seek retreats."},
		{ID: "enchiridion-5", Author: "Epictetus", Work: "Enchiridion", Section: 5, Text: "Men are disturbed."},
	}

	tests := []struct {
		name      string
		offset    int
		err       error
		wantTitle string
		wantBody  string
		wantErr   bool
	}{
		{name: "meditations header", offset: 0, wantTitle: "Marcus Aurelius: Meditations 4.3", wantBody: "Men seek retreats."},
		{name: "enchiridion header has no book", offset: 1, wantTitle: "Epictetus: Enchiridion 5", wantBody: "Men are disturbed."},
		{name: "send failure is returned", offset: 0, err: errors.New("boom"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockSender{err: tt.err}
			err := run(context.Background(), ps, passage.Start.AddDate(0, 0, tt.offset), m)
			if (err != nil) != tt.wantErr {
				t.Fatalf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if m.title != tt.wantTitle || m.body != tt.wantBody {
				t.Errorf("sent (%q, %q), want (%q, %q)", m.title, m.body, tt.wantTitle, tt.wantBody)
			}
		})
	}
}
