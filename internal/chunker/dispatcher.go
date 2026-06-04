package chunker

import (
	"path/filepath"
	"strings"
)

type Dispatcher struct {
	symbol  Chunker
	heading Chunker
	fixed   Chunker
}

func NewDispatcher(symbol, heading, fixed Chunker) *Dispatcher {
	return &Dispatcher{
		symbol:  symbol,
		heading: heading,
		fixed:   fixed,
	}
}

func (d *Dispatcher) Chunk(content string, sourcePath string) []Chunk {
	ext := strings.ToLower(filepath.Ext(sourcePath))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py":
		return d.symbol.Chunk(content, sourcePath)
	case ".md", ".mdx":
		return d.heading.Chunk(content, sourcePath)
	default:
		return d.fixed.Chunk(content, sourcePath)
	}
}
