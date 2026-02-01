package memory

import (
	"encoding/json"
	"fmt"
	"math"
)

// CosineSimilarity returns the cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, magA, magB float64
	for i := 0; i < len(a); i++ {
		vA := float64(a[i])
		vB := float64(b[i])
		dot += vA * vB
		magA += vA * vA
		magB += vB * vB
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

// SerializeEmbedding converts float32 slice to JSON bytes.
func SerializeEmbedding(v []float32) ([]byte, error) {
	return json.Marshal(v)
}

// DeserializeEmbedding converts JSON bytes to float32 slice.
func DeserializeEmbedding(b []byte) ([]float32, error) {
	var v []float32
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("deserialize embedding: %w", err)
	}
	return v, nil
}
