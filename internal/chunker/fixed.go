package chunker

import (
	"github.com/nano-brain/nano-brain/internal/chunk"
)

type FixedChunker struct {
	overlap int
}

func NewFixedChunker() *FixedChunker {
	return &FixedChunker{overlap: 600}
}

func NewFixedChunkerWithOverlap(overlap int) *FixedChunker {
	return &FixedChunker{overlap: overlap}
}

func (f *FixedChunker) Chunk(content string, sourcePath string) []Chunk {
	cfg := chunk.DefaultConfig()
	cfg.Overlap = f.overlap
	raw := chunk.Split(content, cfg)
	out := make([]Chunk, len(raw))
	for i, c := range raw {
		out[i] = Chunk{
			Content:           c.Content,
			Sequence:          c.Sequence,
			StartLine:         c.StartLine,
			EndLine:           c.EndLine,
			Hash:              c.Hash,
			ChunkType:         ChunkTypeRaw,
			EmbeddingStrategy: EmbedStrategyRawCode,
		}
	}
	return out
}
