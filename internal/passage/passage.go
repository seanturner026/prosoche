// Package passage holds the embedded Stoic passages and picks one per day.
package passage

import (
	_ "embed"
	"encoding/json/v2"
	"fmt"
	"strconv"
	"time"
)

// Passage is one numbered section of a work, as written to passages.json by cmd/ingest.
type Passage struct {
	ID         string `json:"id"`
	Author     string `json:"author"`
	Work       string `json:"work"`
	Book       int    `json:"book,omitzero"`
	Section    int    `json:"section"`
	SectionEnd int    `json:"section_end,omitzero"` // set when spanning sections, e.g. 1.1–4
	Text       string `json:"text"`
	WordCount  int    `json:"word_count"`

	Interpretation string `json:"-"` // from interpretations.json; empty if not yet written
}

// Ref is the human citation, e.g. "4.3" or "1.1–4" for Meditations, "5" for the Enchiridion.
func (p Passage) Ref() string {
	ref := strconv.Itoa(p.Section)
	if p.SectionEnd != 0 {
		ref += "–" + strconv.Itoa(p.SectionEnd)
	}
	if p.Book != 0 {
		ref = strconv.Itoa(p.Book) + "." + ref
	}
	return ref
}

// Start is day zero of the reading cycle.
var Start = time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)

var (
	//go:embed passages.json
	rawPassages []byte
	//go:embed interpretations.json
	rawInterpretations []byte
)

// Load decodes the embedded passages.json and attaches interpretations.json by passage ID.
func Load() ([]Passage, error) {
	var ps []Passage
	if err := json.Unmarshal(rawPassages, &ps); err != nil {
		return nil, fmt.Errorf("decode passages.json: %w", err)
	}
	if len(ps) == 0 {
		return nil, fmt.Errorf("passages.json is empty")
	}
	var interps map[string]string
	if err := json.Unmarshal(rawInterpretations, &interps); err != nil {
		return nil, fmt.Errorf("decode interpretations.json: %w", err)
	}
	for i := range ps {
		ps[i].Interpretation = interps[ps[i].ID]
	}
	return ps, nil
}

// ForDate walks ps in order, one per calendar day of t, wrapping after the last.
func ForDate(ps []Passage, t time.Time) Passage {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	days := int(day.Sub(Start).Hours() / 24)
	n := len(ps)
	return ps[((days%n)+n)%n]
}
