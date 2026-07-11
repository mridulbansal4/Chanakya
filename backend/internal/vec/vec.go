// Package vec provides small, dependency-free text embeddings and cosine
// similarity for CHANAKYA's semantic amendment diff. There is no pgvector and
// no external embedding service: over the small in-process corpus, a hashed
// term-frequency embedding with in-Go cosine similarity is correct and fast.
//
// The embedding is deterministic (same text → same vector), so it can be
// computed at compile time, stored as JSON on the obligation, and re-derived
// identically for the amended clause at query time.
package vec

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"regexp"
	"strings"
)

// Dim is the fixed embedding dimensionality. Small is fine at this corpus size;
// 256 keeps hash collisions low over the vocabulary.
const Dim = 256

var wordRe = regexp.MustCompile(`[a-z]+`)

// stopwords are dropped so similarity reflects substantive terms.
var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "of": true,
	"to": true, "in": true, "for": true, "on": true, "at": true, "by": true,
	"with": true, "as": true, "is": true, "are": true, "be": true, "shall": true,
	"must": true, "may": true, "not": true, "any": true, "such": true, "this": true,
	"that": true, "it": true, "its": true, "each": true, "every": true, "who": true,
	"which": true, "from": true, "within": true, "before": true, "after": true,
}

// tokenize lowercases, extracts alpha tokens, drops stopwords, and applies a
// crude singular stem (trailing 's') so "fee"/"fees" align.
func tokenize(text string) []string {
	lower := strings.ToLower(text)
	raw := wordRe.FindAllString(lower, -1)
	out := make([]string, 0, len(raw))
	for _, w := range raw {
		if len(w) < 3 || stopwords[w] {
			continue
		}
		// Crude singular stem so "fee"/"fees", "client"/"clients" align.
		if len(w) > 3 && strings.HasSuffix(w, "s") {
			w = w[:len(w)-1]
		}
		out = append(out, w)
	}
	return out
}

// Embed returns the L2-normalised hashed term-frequency embedding of text. The
// zero vector is returned for text with no substantive tokens.
func Embed(text string) []float64 {
	v := make([]float64, Dim)
	for _, tok := range tokenize(text) {
		h := fnv.New32a()
		_, _ = h.Write([]byte(tok))
		v[h.Sum32()%Dim]++
	}
	var norm float64
	for _, x := range v {
		norm += x * x
	}
	if norm == 0 {
		return v
	}
	norm = math.Sqrt(norm)
	for i := range v {
		v[i] /= norm
	}
	return v
}

// Cosine returns the cosine similarity of a and b (both assumed L2-normalised,
// as produced by Embed). Mismatched lengths or a zero vector yield 0.
func Cosine(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Marshal serialises a vector to compact JSON for storage.
func Marshal(v []float64) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal vector: %w", err)
	}
	return string(b), nil
}

// Unmarshal parses a stored JSON vector. Empty/"[]" input yields a nil slice.
func Unmarshal(s string) ([]float64, error) {
	if s == "" || s == "[]" {
		return nil, nil
	}
	var v []float64
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("unmarshal vector: %w", err)
	}
	return v, nil
}
