package passage_test

import (
	"encoding/json/v2"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/seanturner026/prosoche/internal/passage"
)

func TestLoad(t *testing.T) {
	ps, err := passage.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	books := map[int]int{}
	chapters := 0
	ids := map[string]bool{}
	for _, p := range ps {
		if p.Text == "" || p.WordCount == 0 {
			t.Errorf("%s is empty", p.ID)
		}
		if ids[p.ID] {
			t.Errorf("duplicate id %s", p.ID)
		}
		ids[p.ID] = true
		switch p.Work {
		case "Meditations":
			books[p.Book] += max(p.SectionEnd-p.Section, 0) + 1
		case "Enchiridion":
			chapters++
		}
	}

	if !ids["meditations-1.1-4"] {
		t.Error("missing merged passage meditations-1.1-4")
	}
	if len(books) != 12 {
		t.Errorf("got %d Meditations books, want 12", len(books))
	}
	// Spot-check Long's section counts.
	for book, want := range map[int]int{1: 17, 4: 51, 7: 75, 12: 36} {
		if books[book] != want {
			t.Errorf("Meditations book %d has %d sections, want %d", book, books[book], want)
		}
	}
	// Higginson's edition folds the usual chapters 52-53 into LI.
	if chapters != 51 {
		t.Errorf("got %d Enchiridion chapters, want 51", chapters)
	}
}

// TestPassagesClean guards the committed ingest output against the Gutenberg artifacts that have leaked before.
func TestPassagesClean(t *testing.T) {
	ps, err := passage.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	leaks := []struct {
		name string
		re   *regexp.Regexp
	}{
		{name: "footnote marker", re: regexp.MustCompile(`\[[A-Z]\]|\[\d+\]`)},
		{name: "empty brackets", re: regexp.MustCompile(`\[\s*\]`)},
		{name: "greek or illustration tag", re: regexp.MustCompile(`(?i)\[(?:greek|illustration)`)},
		{name: "transcriber plus mark", re: regexp.MustCompile(`\+`)},
		{name: "cross-reference", re: regexp.MustCompile(`\((?:See\s+|Epictetus,\s+)?[ivxl]+\.\s+\d`)},
		{name: "translator note", re: regexp.MustCompile(`omitted\s+in\s+the\s+translation`)},
		{name: "footnote citation paragraph", re: regexp.MustCompile(`(?m)^(?:See [A-Z]|From the Apologia)`)},
		{name: "italics underscore", re: regexp.MustCompile(`_`)},
		{name: "gutenberg boilerplate", re: regexp.MustCompile(`(?i)gutenberg`)},
		{name: "section number left on text", re: regexp.MustCompile(`^\d+\.\s`)},
		{name: "unnormalized whitespace", re: regexp.MustCompile(`^\s|\s$|  |\t|\n{3,}|[^\n]\n[^\n]`)},
	}
	for _, p := range ps {
		for _, l := range leaks {
			if loc := l.re.FindStringIndex(p.Text); loc != nil {
				start := max(loc[0]-30, 0)
				t.Errorf("%s: %s near %q", p.ID, l.name, p.Text[start:min(loc[1]+30, len(p.Text))])
			}
		}
	}

	// Passages that were truncated or polluted by past ingest bugs.
	byID := map[string]string{}
	for _, p := range ps {
		byID[p.ID] = p.Text
	}
	spots := []struct {
		id, has string
		suffix  bool
	}{
		{id: "meditations-1.1-4", has: "the government of my temper.\n\nFrom the reputation and remembrance of my father"},
		{id: "meditations-5.31", has: `"Never has wronged a man in deed or word."`},
		{id: "meditations-11.6", has: `"Life's harvest reap like the wheat's fruitful ear."`},
		{id: "meditations-7.45", has: "before the baseness [of deserting his post].", suffix: true},
		{id: "meditations-8.57", has: "its rays are called Extensions because they are extended."},
		{id: "meditations-10.23", has: "as in a shepherd's fold on a mountain.", suffix: true},
		{id: "meditations-4.20", has: "a flower, a shrub?", suffix: true},
		{id: "meditations-12.36", has: "for he also who releases thee is satisfied.", suffix: true},
		{id: "enchiridion-51", has: "but hurt me they cannot.”", suffix: true},
	}
	for _, s := range spots {
		text, ok := byID[s.id]
		switch {
		case !ok:
			t.Errorf("%s missing", s.id)
		case s.suffix && !strings.HasSuffix(text, s.has):
			t.Errorf("%s should end with %q, ends %q", s.id, s.has, text[max(len(text)-60, 0):])
		case !s.suffix && !strings.Contains(text, s.has):
			t.Errorf("%s should contain %q", s.id, s.has)
		}
	}
}

func TestInterpretations(t *testing.T) {
	ps, err := passage.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	ids := map[string]bool{}
	for _, p := range ps {
		ids[p.ID] = true
		words := len(strings.Fields(p.Interpretation))
		limit := 150
		if p.WordCount > 300 {
			limit = 250
		}
		switch {
		case words == 0:
			t.Errorf("%s has no interpretation", p.ID)
		case words > limit:
			t.Errorf("%s interpretation is %d words, want <= %d", p.ID, words, limit)
		}
	}

	b, err := os.ReadFile("interpretations.json")
	if err != nil {
		t.Fatal(err)
	}
	var interps map[string]string
	if err := json.Unmarshal(b, &interps); err != nil {
		t.Fatal(err)
	}
	for id := range interps {
		if !ids[id] {
			t.Errorf("interpretations.json has %s, which is not in passages.json", id)
		}
	}
}

func TestRef(t *testing.T) {
	tests := []struct {
		name string
		p    passage.Passage
		want string
	}{
		{name: "book and section", p: passage.Passage{Book: 4, Section: 3}, want: "4.3"},
		{name: "section range", p: passage.Passage{Book: 1, Section: 1, SectionEnd: 4}, want: "1.1–4"},
		{name: "chapter only", p: passage.Passage{Section: 5}, want: "5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Ref(); got != tt.want {
				t.Errorf("Ref() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestForDate(t *testing.T) {
	ps := []passage.Passage{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	ny, _ := time.LoadLocation("America/New_York")
	day := func(offset int) time.Time {
		return passage.Start.AddDate(0, 0, offset)
	}

	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{name: "start date is first passage", at: day(0), want: "a"},
		{name: "next day is next passage", at: day(1), want: "b"},
		{name: "wraps after last passage", at: day(3), want: "a"},
		{name: "before start wraps backwards", at: day(-1), want: "c"},
		{name: "time of day is ignored", at: time.Date(2026, 10, 20, 23, 59, 0, 0, ny), want: "b"},
		{name: "day after DST ends", at: time.Date(2026, 11, 2, 7, 40, 0, 0, ny), want: "c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := passage.ForDate(ps, tt.at).ID; got != tt.want {
				t.Errorf("ForDate(%s) = %s, want %s", tt.at, got, tt.want)
			}
		})
	}
}
