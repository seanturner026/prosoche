package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/seanturner026/prosoche/internal/passage"
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
		{name: "transcriber plus marks", in: "[+ Neither actor+]. spin thy thread + into", want: "[Neither actor]. spin thy thread into"},
		{name: "double-bracketed greek", in: "called Extensions [[Greek: aktines]] because", want: "called Extensions because"},
		{name: "translator note", in: "on a mountain. [The three last words are omitted\nin the translation.]", want: "on a mountain."},
		{name: "illustration", in: "a shrub?\n\n[Illustration: THE FORUM]", want: "a shrub?"},
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
// section, footnotes (indented or flush), indented verse, and multi-paragraph sections.
func meditationsFixture(books int) string {
	romans := []string{"I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X", "XI", "XII"}
	var b strings.Builder
	for i := range books {
		fmt.Fprintf(&b, "%s.\n\n\nFirst thought of book %d.[A]\n\n", romans[i], i+1)
		b.WriteString("    [A] A footnote that\n    spans lines.\n\n")
		b.WriteString("    Its second paragraph, also indented.\n\n")
		b.WriteString("[B] A flush footnote.\n\n")
		b.WriteString("2. Second thought,\nwrapped.\n\nIts second paragraph.\n\n")
		b.WriteString("3. Third thought (ii. 1).\n\n")
		b.WriteString("5. Not a section heading, since 4 never appeared.\n\n")
		b.WriteString("      \"Indented verse in the main text.\"\n\n")
	}
	return b.String()
}

func TestParseMeditations(t *testing.T) {
	ps, err := parseMeditations(meditationsFixture(12))
	if err != nil {
		t.Fatalf("parseMeditations() error: %v", err)
	}
	byID := map[string]passage.Passage{}
	for _, p := range ps {
		byID[p.ID] = p
	}
	// Book 1's sections 1-3 fold into one via meditationsMerges.
	if len(ps) != 34 {
		t.Fatalf("got %d passages, want 34", len(ps))
	}

	got := byID["meditations-12.2"]
	if got.Book != 12 || got.Section != 2 {
		t.Errorf("meditations-12.2 = book %d section %d", got.Book, got.Section)
	}
	if want := "Second thought, wrapped.\n\nIts second paragraph."; got.Text != want {
		t.Errorf("text = %q, want %q", got.Text, want)
	}
	if got.WordCount != 6 {
		t.Errorf("word_count = %d, want 6", got.WordCount)
	}
	if want := "First thought of book 2."; byID["meditations-2.1"].Text != want {
		t.Errorf("unnumbered first section = %q, want %q", byID["meditations-2.1"].Text, want)
	}
	if want := "Third thought.\n\n5. Not a section heading, since 4 never appeared.\n\n\"Indented verse in the main text.\""; byID["meditations-2.3"].Text != want {
		t.Errorf("out-of-sequence number = %q, want %q", byID["meditations-2.3"].Text, want)
	}
	if merged := byID["meditations-1.1-4"]; merged.SectionEnd != 4 || !strings.HasPrefix(merged.Text, "First thought of book 1.\n\nSecond thought") {
		t.Errorf("merged book 1 = %+v", merged)
	}

	if _, err := parseMeditations(meditationsFixture(11)); err == nil {
		t.Error("parseMeditations() with 11 books: want error")
	}
}

func TestMerge(t *testing.T) {
	units := []unit{
		{book: 1, section: 1, paras: []string{"a"}},
		{book: 1, section: 2, paras: []string{"b"}},
		{book: 1, section: 3, paras: []string{"c"}},
		{book: 2, section: 1, paras: []string{"d"}},
		{book: 2, section: 2, paras: []string{"e"}},
	}
	got := merge(units, []span{{book: 1, from: 1, to: 2}})

	want := []unit{
		{book: 1, section: 1, end: 2, paras: []string{"a", "b"}},
		{book: 1, section: 3, paras: []string{"c"}},
		{book: 2, section: 1, paras: []string{"d"}},
		{book: 2, section: 2, paras: []string{"e"}},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("merge() = %v, want %v", got, want)
	}
}

func TestInterleave(t *testing.T) {
	ps := func(ids ...string) []passage.Passage {
		out := make([]passage.Passage, len(ids))
		for i, id := range ids {
			out[i] = passage.Passage{ID: id}
		}
		return out
	}
	tests := []struct {
		name string
		a, b []passage.Passage
		want string
	}{
		{name: "spreads b evenly", a: ps("a1", "a2", "a3", "a4", "a5", "a6"), b: ps("b1", "b2"), want: "a1 a2 b1 a3 a4 a5 b2 a6"},
		{name: "equal lengths alternate", a: ps("a1", "a2"), b: ps("b1", "b2"), want: "a1 b1 a2 b2"},
		{name: "empty b", a: ps("a1", "a2"), want: "a1 a2"},
		{name: "empty a", b: ps("b1", "b2"), want: "b1 b2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ids []string
			for _, p := range interleave(tt.a, tt.b) {
				ids = append(ids, p.ID)
			}
			if got := strings.Join(ids, " "); got != tt.want {
				t.Errorf("interleave() = %s, want %s", got, tt.want)
			}
		})
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
