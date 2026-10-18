package passage_test

import (
	"encoding/json/v2"
	"os"
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
