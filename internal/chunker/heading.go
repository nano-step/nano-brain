package chunker

import (
	"github.com/nano-brain/nano-brain/internal/chunk"
)

type HeadingChunker struct{}

func NewHeadingChunker() *HeadingChunker {
	return &HeadingChunker{}
}

func (h *HeadingChunker) Chunk(content string, sourcePath string) []Chunk {
	raw := chunk.Split(content, chunk.DefaultConfig())
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
