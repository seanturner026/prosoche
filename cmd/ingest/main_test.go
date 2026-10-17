package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestStripGutenberg(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{
			name: "drops header, footer and license",
			in: "The Project Gutenberg eBook of X\nTitle: X\n\n" +
				"*** START OF THE PROJECT GUTENBERG EBOOK X ***\nbody line\n\n" +
				"*** END OF THE PROJECT GUTENBERG EBOOK X ***\n1.A. By reading or using any part of this Project Gutenberg",
			want: "body line\n\n",
		},
		{name: "missing start marker", in: "body\n*** END OF X ***", wantErr: true},
		{name: "missing end marker", in: "*** START OF X ***\nbody", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stripGutenberg(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("stripGutenberg() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("stripGutenberg() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBetween(t *testing.T) {
	in := "INTRODUCTION\nI.\nsee the I. below\n\nI.\n\nbody\n\nINDEXES.\nindex"
	got, err := between(in, "I.", "INDEXES.")
	if err != nil {
		t.Fatalf("between() error: %v", err)
	}
	if want := "I.\nsee the I. below\n\nI.\n\nbody\n"; got != want {
		t.Errorf("between() = %q, want %q", got, want)
	}
	if _, err := between(in, "I.", "MISSING"); err == nil {
		t.Error("between() with missing end line: want error")
	}
}

func TestClean(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "letter footnote marker", in: "my grandfather Verus[A] [I learned]", want: "my grandfather Verus [I learned]"},
		{name: "numeric footnote marker", in: "follow still.[8]", want: "follow still."},
		{name: "cross reference", in: "a terrible thing (iii. 7; vi.\n23; x. 20).", want: "a terrible thing."},
		{name: "epictetus reference", in: "exists not yet (Epictetus,\niii. 24).", want: "exists not yet."},
		{name: "greek transliteration", in: "a part [Greek: meros], thou", want: "a part, thou"},
		{name: "italics and whitespace", in: "  as, _We ought\n  not to lie_;  ", want: "as, We ought not to lie;"},
		{name: "keeps ordinary parentheses", in: "the man (who is good) acts", want: "the man (who is good) acts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clean(tt.in); got != tt.want {
				t.Errorf("clean(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// meditationsFixture mimics Long's layout: roman book headings, an unnumbered first
// section, indented footnotes, and multi-paragraph sections.
func meditationsFixture(books int) string {
	romans := []string{"I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X", "XI", "XII"}
	var b strings.Builder
	for i := range books {
		fmt.Fprintf(&b, "%s.\n\n\nFirst thought of book %d.[A]\n\n", romans[i], i+1)
		b.WriteString("    [A] A footnote that\n    spans lines.\n\n")
		b.WriteString("2. Second thought,\nwrapped.\n\nIts second paragraph.\n\n")
		b.WriteString("3. Third thought (ii. 1).\n\n")
		b.WriteString("5. Not a section heading, since 4 never appeared.\n\n")
	}
	return b.String()
}

func TestParseMeditations(t *testing.T) {
	ps, err := parseMeditations(meditationsFixture(12))
	if err != nil {
		t.Fatalf("parseMeditations() error: %v", err)
	}
	if len(ps) != 36 {
		t.Fatalf("got %d passages, want 36", len(ps))
	}

	got := ps[3*11+1]
	if got.ID != "meditations-12.2" || got.Book != 12 || got.Section != 2 {
		t.Errorf("passage = %s (book %d section %d), want meditations-12.2", got.ID, got.Book, got.Section)
	}
	if want := "Second thought, wrapped.\n\nIts second paragraph."; got.Text != want {
		t.Errorf("text = %q, want %q", got.Text, want)
	}
	if got.WordCount != 6 {
		t.Errorf("word_count = %d, want 6", got.WordCount)
	}
	if want := "First thought of book 1."; ps[0].Text != want {
		t.Errorf("unnumbered first section = %q, want %q", ps[0].Text, want)
	}
	if want := "Third thought.\n\n5. Not a section heading, since 4 never appeared."; ps[2].Text != want {
		t.Errorf("out-of-sequence number = %q, want %q", ps[2].Text, want)
	}

	if _, err := parseMeditations(meditationsFixture(11)); err == nil {
		t.Error("parseMeditations() with 11 books: want error")
	}
}

func TestParseEnchiridion(t *testing.T) {
	in := "                            THE ENCHIRIDION\n\n\n" +
		"                                   I\n\nThere are things within our power.\n\nSecond paragraph.\n\n\n" +
		"                                 II[2]\n\nUpon all occasions:[1]\n\n" +
		"  Conduct me, Zeus,\n  and thou, O Destiny.\n"
	ps, err := parseEnchiridion(in)
	if err != nil {
		t.Fatalf("parseEnchiridion() error: %v", err)
	}
	if len(ps) != 2 {
		t.Fatalf("got %d chapters, want 2", len(ps))
	}
	if ps[0].ID != "enchiridion-1" || ps[0].Book != 0 || ps[0].Section != 1 {
		t.Errorf("first = %+v, want enchiridion-1", ps[0])
	}
	if want := "There are things within our power.\n\nSecond paragraph."; ps[0].Text != want {
		t.Errorf("chapter I = %q, want %q", ps[0].Text, want)
	}
	if want := "Upon all occasions:\n\nConduct me, Zeus, and thou, O Destiny."; ps[1].Text != want {
		t.Errorf("chapter II = %q, want %q", ps[1].Text, want)
	}

	if _, err := parseEnchiridion("no chapters here"); err == nil {
		t.Error("parseEnchiridion() without chapters: want error")
	}
}

func TestHeading(t *testing.T) {
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{in: "XII.", want: 12, ok: true},
		{in: "      XLIX", want: 49, ok: true},
		{in: "  XXIX[2]", want: 29, ok: true},
		{in: "I learned", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := heading(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Errorf("heading(%q) = %d, %v, want %d, %v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}
