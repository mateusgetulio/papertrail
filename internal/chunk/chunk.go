package chunk

import "strings"

const (
	DefaultMaxChars = 4800  // ~1200 tokens at 4 chars/token
	DefaultOverlap  = 720   // ~15% overlap
)

// Chunk splits text into overlapping segments suitable for LLM context windows.
// It respects paragraph boundaries where possible.
func Chunk(text string, maxChars, overlap int) []string {
	if maxChars <= 0 {
		maxChars = DefaultMaxChars
	}
	if overlap <= 0 {
		overlap = DefaultOverlap
	}

	paragraphs := splitParagraphs(text)
	var chunks []string
	var current strings.Builder

	flush := func() {
		s := strings.TrimSpace(current.String())
		if s != "" {
			chunks = append(chunks, s)
		}
		current.Reset()
	}

	for _, para := range paragraphs {
		if para == "" {
			continue
		}
		if current.Len()+len(para)+1 > maxChars {
			flush()
			// Carry overlap from the tail of the previous chunk.
			if len(chunks) > 0 {
				prev := chunks[len(chunks)-1]
				tail := prev
				if len(tail) > overlap {
					tail = tail[len(tail)-overlap:]
				}
				current.WriteString(tail)
				current.WriteRune('\n')
			}
		}
		current.WriteString(para)
		current.WriteRune('\n')
	}
	flush()
	return chunks
}

func splitParagraphs(text string) []string {
	lines := strings.Split(text, "\n")
	var paras []string
	var buf strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if buf.Len() > 0 {
				paras = append(paras, buf.String())
				buf.Reset()
			}
		} else {
			if buf.Len() > 0 {
				buf.WriteRune(' ')
			}
			buf.WriteString(trimmed)
		}
	}
	if buf.Len() > 0 {
		paras = append(paras, buf.String())
	}
	return paras
}
