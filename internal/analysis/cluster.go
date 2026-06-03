package analysis

import "math"

// CosineSimilarity returns the cosine similarity between two equal-length vectors.
// Returns 0 if either vector is nil or zero-length.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// IsDuplicate returns true if the candidate embedding is too similar (≥ threshold)
// to any existing embedding.
func IsDuplicate(candidate []float32, existing []float32, threshold float64) bool {
	return CosineSimilarity(candidate, existing) >= threshold
}

const DedupThreshold = 0.86 // docs/07 §5
