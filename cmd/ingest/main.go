package main

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/seanturner026/prosoche/internal/passage"
)

const (
	meditationsURL = "https://www.gutenberg.org/ebooks/15877.txt.utf-8"
	enchiridionURL = "https://www.gutenberg.org/ebooks/45109.txt.utf-8"
)

var (
	paragraphRe = regexp.MustCompile(`\n[ \t]*\n`)
	sectionRe   = regexp.MustCompile(`^(\d+)\.\s+`)
	romanRe     = regexp.MustCompile(`^([IVXL]+)\.?(?:\[\d+\])?$`)

	// Editorial apparatus: footnote markers ([A], [1], +), Greek transliterations, and Long's
	// cross-references like "(iii. 7; vi. 23)" or "(Epictetus, iii. 24)".
	noiseRes = []*regexp.Regexp{
		regexp.MustCompile(`\[[A-Z]\]|\[\d+\]|\+`),
		regexp.MustCompile(`\s*\[Greek:[^\]]*\]`),
		regexp.MustCompile(`\s*\((?:See\s+|Epictetus,\s+)?[ivxl]+\.\s+[^()]*\d[^()]*\)`),
	}
	whitespaceRe = regexp.MustCompile(`\s+`)
)

func fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get %s returned %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n"), nil
}

// stripGutenberg returns the text between the "*** START OF" and "*** END OF" lines,
// dropping the Project Gutenberg header, footer, and license.
func stripGutenberg(s string) (string, error) {
	start := strings.Index(s, "*** START OF")
	end := strings.Index(s, "*** END OF")
	if start == -1 || end == -1 || end < start {
		return "", errors.New("gutenberg start/end markers not found")
	}
	start += strings.IndexByte(s[start:], '\n') + 1
	return s[start:end], nil
}

// between returns s from the first line equal to startLine up to the first later line
// equal to endLine, comparing trimmed lines. It cuts translator front and back matter.
func between(s, startLine, endLine string) (string, error) {
	lines := strings.Split(s, "\n")
	start := slices.IndexFunc(lines, func(l string) bool { return strings.TrimSpace(l) == startLine })
	if start == -1 {
		return "", fmt.Errorf("line %q not found", startLine)
	}
	end := slices.IndexFunc(lines[start+1:], func(l string) bool { return strings.TrimSpace(l) == endLine })
	if end == -1 {
		return "", fmt.Errorf("line %q not found after %q", endLine, startLine)
	}
	return strings.Join(lines[start:start+1+end], "\n"), nil
}

func paragraphs(s string) []string {
	var out []string
	for _, p := range paragraphRe.Split(s, -1) {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

func roman(s string) int {
	vals := map[byte]int{'I': 1, 'V': 5, 'X': 10, 'L': 50}
	n := 0
	for i := range len(s) {
		v := vals[s[i]]
		if i+1 < len(s) && v < vals[s[i+1]] {
			n -= v
		} else {
			n += v
		}
	}
	return n
}

// heading returns the number of a roman-numeral heading paragraph like "IV." or "XXIX[2]".
func heading(p string) (int, bool) {
	m := romanRe.FindStringSubmatch(strings.TrimSpace(p))
	if m == nil {
		return 0, false
	}
	return roman(m[1]), true
}

func clean(p string) string {
	for _, re := range noiseRes {
		p = re.ReplaceAllString(p, "")
	}
	p = strings.ReplaceAll(p, "_", "")
	p = whitespaceRe.ReplaceAllString(p, " ")
	return strings.TrimSpace(strings.ReplaceAll(p, "[ ", "["))
}

type unit struct {
	book, section, end int
	paras              []string
}

// span is an inclusive range of sections in one book that only read well as one passage.
type span struct{ book, from, to int }

var meditationsMerges = []span{
	{book: 1, from: 1, to: 4},    // family list; its verb "[I learned]" appears only in 1.1
	{book: 7, from: 38, to: 43},  // run of one-line quotations from the poets
	{book: 11, from: 30, to: 32}, // more quotations, two ending in Long's source citation
	{book: 11, from: 37, to: 38}, // "he said" in 11.38 is the Epictetus named in 11.37
}

// merge folds each span's sections into the unit for its first section.
func merge(units []unit, spans []span) []unit {
	var out []unit
	for _, u := range units {
		if n := len(out); n > 0 {
			last := &out[n-1]
			if last.book == u.book && last.section < u.section && u.section <= last.end {
				last.paras = append(last.paras, u.paras...)
				continue
			}
		}
		for _, s := range spans {
			if s.book == u.book && s.from == u.section {
				u.end = s.to
			}
		}
		out = append(out, u)
	}
	return out
}

func toPassages(units []unit, author, work, slug string) []passage.Passage {
	ps := make([]passage.Passage, 0, len(units))
	for _, u := range units {
		var paras []string
		for _, p := range u.paras {
			if c := clean(p); c != "" {
				paras = append(paras, c)
			}
		}
		text := strings.Join(paras, "\n\n")
		p := passage.Passage{
			Author:     author,
			Work:       work,
			Book:       u.book,
			Section:    u.section,
			SectionEnd: u.end,
			Text:       text,
			WordCount:  len(strings.Fields(text)),
		}
		p.ID = slug + "-" + strings.ReplaceAll(p.Ref(), "–", "-")
		ps = append(ps, p)
	}
	return ps
}

// parseMeditations splits Long's text into sections. Books open with a roman heading
// ("IV."), their first section is unnumbered, and later ones start "2. ", "3. ", ...
func parseMeditations(body string) ([]passage.Passage, error) {
	var units []unit
	book, section := 0, 0
	for _, p := range paragraphs(body) {
		if n, ok := heading(p); ok && n == book+1 {
			book, section = n, 0
			continue
		}
		if book == 0 {
			continue
		}
		// Long's footnotes are the only indented paragraphs.
		if strings.HasPrefix(p, "    ") {
			continue
		}
		if m := sectionRe.FindStringSubmatch(p); m != nil {
			if n, _ := strconv.Atoi(m[1]); n == section+1 {
				section = n
				units = append(units, unit{book: book, section: section, paras: []string{p[len(m[0]):]}})
				continue
			}
		}
		if section == 0 {
			section = 1
			units = append(units, unit{book: book, section: section, paras: []string{p}})
			continue
		}
		units[len(units)-1].paras = append(units[len(units)-1].paras, p)
	}
	if book != 12 {
		return nil, fmt.Errorf("meditations: found %d books, want 12", book)
	}
	return toPassages(merge(units, meditationsMerges), "Marcus Aurelius", "Meditations", "meditations"), nil
}

// parseEnchiridion splits Higginson's text on centered roman chapter headings.
func parseEnchiridion(body string) ([]passage.Passage, error) {
	var units []unit
	for _, p := range paragraphs(body) {
		if n, ok := heading(p); ok && n == len(units)+1 {
			units = append(units, unit{section: n})
			continue
		}
		if len(units) == 0 {
			continue
		}
		units[len(units)-1].paras = append(units[len(units)-1].paras, p)
	}
	if len(units) == 0 {
		return nil, errors.New("enchiridion: no chapters found")
	}
	return toPassages(units, "Epictetus", "Enchiridion", "enchiridion"), nil
}

// interleave spreads b evenly through a, keeping each in its own order. The j-th b lands
// after (j+½)·len(a)/len(b) of a, so neither work bunches up at the start or end.
func interleave(a, b []passage.Passage) []passage.Passage {
	out := make([]passage.Passage, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		if j < len(b) && (i == len(a) || (2*j+1)*len(a) <= 2*i*len(b)) {
			out = append(out, b[j])
			j++
		} else {
			out = append(out, a[i])
			i++
		}
	}
	return out
}

func histogram(ps []passage.Passage) string {
	edges := []int{25, 50, 100, 150, 200, 300, 500, 1000}
	counts := make([]int, len(edges)+1)
	for _, p := range ps {
		i, _ := slices.BinarySearch(edges, p.WordCount+1)
		counts[i]++
	}

	var b strings.Builder
	lo := 0
	for i, c := range counts {
		label := fmt.Sprintf("%4d+", lo)
		if i < len(edges) {
			label = fmt.Sprintf("%4d-%-4d", lo, edges[i]-1)
			lo = edges[i]
		}
		fmt.Fprintf(&b, "%-9s %4d %s\n", label, c, strings.Repeat("#", (c+4)/5))
	}

	wcs := make([]int, len(ps))
	for i, p := range ps {
		wcs[i] = p.WordCount
	}
	slices.Sort(wcs)
	fmt.Fprintf(&b, "\n%d passages, words min %d / median %d / max %d\n", len(ps), wcs[0], wcs[len(wcs)/2], wcs[len(wcs)-1])
	return b.String()
}

func load(ctx context.Context, url, startLine, endLine string, parse func(string) ([]passage.Passage, error)) ([]passage.Passage, error) {
	raw, err := fetch(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	text, err := stripGutenberg(raw)
	if err != nil {
		return nil, err
	}
	body, err := between(text, startLine, endLine)
	if err != nil {
		return nil, err
	}
	return parse(body)
}

func run(ctx context.Context, out string) error {
	meditations, err := load(ctx, meditationsURL, "I.", "INDEXES.", parseMeditations)
	if err != nil {
		return fmt.Errorf("meditations: %w", err)
	}
	enchiridion, err := load(ctx, enchiridionURL, "THE ENCHIRIDION", "Footnotes", parseEnchiridion)
	if err != nil {
		return fmt.Errorf("enchiridion: %w", err)
	}
	ps := interleave(meditations, enchiridion)

	b, err := json.Marshal(ps, jsontext.WithIndent("  "))
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(out, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}

	fmt.Print(histogram(ps))
	fmt.Printf("meditations %d, enchiridion %d -> %s\n", len(meditations), len(enchiridion), out)
	return nil
}

func main() {
	out := flag.String("out", "internal/passage/passages.json", "output path")
	flag.Parse()

	if err := run(context.Background(), *out); err != nil {
		slog.Error("ingest failed", "error", err)
		os.Exit(1)
	}
}
