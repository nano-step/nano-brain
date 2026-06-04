package chunker

type ChunkType string

const (
	ChunkTypeRaw    ChunkType = "raw"
	ChunkTypeSymbol ChunkType = "symbol"
)

type EmbeddingStrategy string

const (
	EmbedStrategyRawCode  EmbeddingStrategy = "raw_code"
	EmbedStrategySymbol   EmbeddingStrategy = "symbol_code"
)

type Chunk struct {
	Content           string
	Sequence          int
	StartLine         int
	EndLine           int
	Hash              string
	SymbolName        string
	SymbolKind        string
	Language          string
	ChunkType         ChunkType
	EmbeddingStrategy EmbeddingStrategy
}

type Chunker interface {
	Chunk(content string, sourcePath string) []Chunk
}
