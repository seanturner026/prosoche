package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
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

// fakeDiscord records posts by path and answers each with a sequential ID.
type fakeDiscord struct {
	posts       map[string][]string
	failThreads bool
	n           int
}

func (f *fakeDiscord) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bot tok" || (f.failThreads && strings.HasSuffix(r.URL.Path, "/threads")) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	b, _ := io.ReadAll(r.Body)
	var payload map[string]string
	_ = json.Unmarshal(b, &payload)
	f.posts[r.URL.Path] = append(f.posts[r.URL.Path], payload["content"]+payload["name"])
	f.n++
	fmt.Fprintf(w, `{"id":"id%d"}`, f.n)
}

func TestDiscordSend(t *testing.T) {
	long := strings.Repeat("word ", 300) + "\n\n" + strings.Repeat("more ", 300)

	tests := []struct {
		name        string
		token       string
		msg         Message
		failThreads bool
		want        map[string][]string
		wantErr     error
	}{
		{
			name:  "long passage with reply threads under last post",
			token: "tok",
			msg:   Message{Title: "Epictetus: Enchiridion 1", Body: long, Reply: "It means this."},
			want: map[string][]string{
				"/channels/chan1/messages":             {"**Epictetus: Enchiridion 1**\n\n" + strings.TrimSpace(strings.Repeat("word ", 300)), strings.TrimSpace(strings.Repeat("more ", 300))},
				"/channels/chan1/messages/id2/threads": {"Epictetus: Enchiridion 1"},
				"/channels/id3/messages":               {"It means this."},
			},
		},
		{
			name:  "no reply means no thread",
			token: "tok",
			msg:   Message{Title: "t", Body: "b"},
			want:  map[string][]string{"/channels/chan1/messages": {"**t**\n\nb"}},
		},
		{
			name:        "thread failure is ErrReply",
			token:       "tok",
			msg:         Message{Title: "t", Body: "b", Reply: "r"},
			failThreads: true,
			want:        map[string][]string{"/channels/chan1/messages": {"**t**\n\nb"}},
			wantErr:     ErrReply,
		},
		{
			name:  "rejected token is a plain error",
			token: "wrong",
			msg:   Message{Title: "t", Body: "b", Reply: "r"},
			want:  map[string][]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeDiscord{posts: map[string][]string{}, failThreads: tt.failThreads}
			srv := httptest.NewServer(f)
			defer srv.Close()

			d := discordSender{baseURL: srv.URL, channelID: "chan1", botToken: tt.token}
			err := d.Send(context.Background(), tt.msg)
			switch {
			case tt.token != "tok":
				if err == nil || errors.Is(err, ErrReply) {
					t.Errorf("Send() error = %v, want non-reply error", err)
				}
			case !errors.Is(err, tt.wantErr):
				t.Errorf("Send() error = %v, want %v", err, tt.wantErr)
			}
			if fmt.Sprint(f.posts) != fmt.Sprint(tt.want) {
				t.Errorf("posts = %q, want %q", f.posts, tt.want)
			}
		})
	}
}

type mockSender struct {
	got Message
	err error
}

func (m *mockSender) Send(_ context.Context, msg Message) error {
	m.got = msg
	return m.err
}

func TestRun(t *testing.T) {
	ps := []passage.Passage{
		{ID: "meditations-4.3", Author: "Marcus Aurelius", Work: "Meditations", Book: 4, Section: 3, Text: "Men seek retreats.", Interpretation: "Retreat inward."},
		{ID: "enchiridion-5", Author: "Epictetus", Work: "Enchiridion", Section: 5, Text: "Men are disturbed."},
	}

	tests := []struct {
		name    string
		offset  int
		err     error
		want    Message
		wantErr bool
	}{
		{name: "meditations header and reply", offset: 0, want: Message{Title: "Marcus Aurelius: Meditations 4.3", Body: "Men seek retreats.", Reply: "Retreat inward."}},
		{name: "enchiridion header has no book", offset: 1, want: Message{Title: "Epictetus: Enchiridion 5", Body: "Men are disturbed."}},
		{name: "reply failure is not retried", offset: 0, err: fmt.Errorf("%w: boom", ErrReply), want: Message{Title: "Marcus Aurelius: Meditations 4.3", Body: "Men seek retreats.", Reply: "Retreat inward."}},
		{name: "send failure is returned", offset: 0, err: errors.New("boom"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockSender{err: tt.err}
			err := run(context.Background(), ps, passage.Start.AddDate(0, 0, tt.offset), m)
			if (err != nil) != tt.wantErr {
				t.Fatalf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && m.got != tt.want {
				t.Errorf("sent %+v, want %+v", m.got, tt.want)
			}
		})
	}
}
