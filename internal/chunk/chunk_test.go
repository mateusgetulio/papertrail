package chunk

import (
	"reflect"
	"strings"
	"testing"
)

func TestChunk(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		maxChars int
		overlap  int
		want     []string
	}{
		{
			name:     "empty input yields no chunks",
			text:     "",
			maxChars: 10,
			overlap:  3,
			want:     nil,
		},
		{
			name:     "whitespace only yields no chunks",
			text:     "\n\n   \n\t\n",
			maxChars: 10,
			overlap:  3,
			want:     nil,
		},
		{
			name:     "short text fits in one chunk",
			text:     "hello world",
			maxChars: 100,
			overlap:  10,
			want:     []string{"hello world"},
		},
		{
			name:     "zero limits fall back to defaults",
			text:     "hello",
			maxChars: 0,
			overlap:  0,
			want:     []string{"hello"},
		},
		{
			name:     "lines within a paragraph join with spaces",
			text:     "line one\nline two\n\nnext para",
			maxChars: 100,
			overlap:  10,
			want:     []string{"line one line two\nnext para"},
		},
		{
			name:     "splits at max and carries overlap tail",
			text:     "aaaa\n\nbbbb\n\ncccc",
			maxChars: 10,
			overlap:  3,
			want:     []string{"aaaa\nbbbb", "bbb\ncccc"},
		},
		{
			name:     "oversized single paragraph kept whole",
			text:     strings.Repeat("x", 20),
			maxChars: 10,
			overlap:  3,
			want:     []string{strings.Repeat("x", 20)},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Chunk(c.text, c.maxChars, c.overlap)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Chunk(%q, %d, %d) = %q, want %q", c.text, c.maxChars, c.overlap, got, c.want)
			}
		})
	}
}

func TestChunkOverlapContinuity(t *testing.T) {
	var paras []string
	for i := 0; i < 10; i++ {
		paras = append(paras, strings.Repeat(string(rune('a'+i)), 400))
	}
	text := strings.Join(paras, "\n\n")

	chunks := Chunk(text, 1000, 100)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i := 1; i < len(chunks); i++ {
		prev := chunks[i-1]
		tail := prev[len(prev)-100:]
		if !strings.HasPrefix(chunks[i], tail) {
			t.Errorf("chunk %d does not start with the 100-char tail of chunk %d", i, i-1)
		}
	}
}

func TestSplitParagraphs(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"empty", "", nil},
		{"single line", "solo", []string{"solo"}},
		{"blank line separates paragraphs", "a\nb\n\nc", []string{"a b", "c"}},
		{"multiple blank lines collapse", "a\n\n\n\nb", []string{"a", "b"}},
		{"surrounding whitespace trimmed", "  a  \n\t\n  b  ", []string{"a", "b"}},
		{"trailing paragraph without newline kept", "a\n\nb", []string{"a", "b"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := splitParagraphs(c.text)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("splitParagraphs(%q) = %q, want %q", c.text, got, c.want)
			}
		})
	}
}
