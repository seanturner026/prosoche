// Package passage holds the embedded Stoic passages and picks one per day.
package passage

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/go-json-experiment/json"
)

// Passage is one numbered section of a work, as written to passages.json by cmd/ingest.
type Passage struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Work      string `json:"work"`
	Book      int    `json:"book,omitzero"`
	Section   int    `json:"section"`
	Text      string `json:"text"`
	WordCount int    `json:"word_count"`
}

// Ref is the human citation, e.g. "4.3" for Meditations or "5" for the Enchiridion.
func (p Passage) Ref() string {
	if p.Book == 0 {
		return fmt.Sprintf("%d", p.Section)
	}
	return fmt.Sprintf("%d.%d", p.Book, p.Section)
}

// Start is day zero of the reading cycle.
var Start = time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)

//go:embed passages.json
var raw []byte

// Load decodes the embedded passages.json.
func Load() ([]Passage, error) {
	var ps []Passage
	if err := json.Unmarshal(raw, &ps); err != nil {
		return nil, fmt.Errorf("decode passages.json: %w", err)
	}
	if len(ps) == 0 {
		return nil, fmt.Errorf("passages.json is empty")
	}
	return ps, nil
}

// ForDate walks ps in file order, one per calendar day of t, wrapping after the last.
func ForDate(ps []Passage, t time.Time) Passage {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	days := int(day.Sub(Start).Hours() / 24)
	n := len(ps)
	return ps[((days%n)+n)%n]
}
