package store

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"time"
)

type MemoryChunk struct {
	ID        int64
	Content   string
	Embedding []float32
	Source    string
	CreatedAt time.Time
	Score     float64 // Similarity score (transient)
}

// InsertChunk saves a memory chunk with its embedding.
func (db *DB) InsertChunk(ctx context.Context, content string, source string, embedding []float32) error {
	embBytes, err := json.Marshal(embedding)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, 
		`INSERT INTO memory_chunks (content, source, embedding) VALUES (?, ?, ?)`,
		content, source, embBytes,
	)
	return err
}

// SearchChunks performs a naive vector search (cosine similarity).
// Note: This fetches ALL chunks. For scale > 10k, use sqlite-vec or separate vector DB.
func (db *DB) SearchChunks(ctx context.Context, queryEmb []float32, limit int) ([]MemoryChunk, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, content, embedding, source, created_at FROM memory_chunks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []MemoryChunk

	for rows.Next() {
		var c MemoryChunk
		var embBytes []byte
		if err := rows.Scan(&c.ID, &c.Content, &embBytes, &c.Source, &c.CreatedAt); err != nil {
			return nil, err
		}
		if len(embBytes) > 0 {
			if err := json.Unmarshal(embBytes, &c.Embedding); err == nil {
				score := cosineSimilarity(queryEmb, c.Embedding)
				c.Score = score
				candidates = append(candidates, c)
			}
		}
	}

	// Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if len(candidates) > limit {
		return candidates[:limit], nil
	}
	return candidates, nil
}

func cosineSimilarity(a, b []float32) float64 {
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
