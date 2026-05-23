// Package embed handles vector embedding generation.
package embed

import "context"

// Embedder generates vector embeddings from text.
type Embedder interface {
	// Embed returns a float32 vector for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)
	// Dimension returns the expected vector dimension.
	Dimension() int
}
