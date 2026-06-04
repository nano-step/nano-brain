package chunker

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/rs/zerolog"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const maxSymbolBytes = 8192

type langConfig struct {
	lang  *gotreesitter.Language
	query string
}

type SymbolAwareChunker struct {
	fallback Chunker
	logger   zerolog.Logger
	langs    map[string]*parsedLang
}

type parsedLang struct {
	lang  *gotreesitter.Language
	query *gotreesitter.Query
}

type symbolSpan struct {
	name      string
	kind      string
	startByte uint32
	endByte   uint32
}

func NewSymbolAwareChunker(fallback Chunker, logger zerolog.Logger) (*SymbolAwareChunker, error) {
	configs := map[string]langConfig{
		".go": {lang: grammars.GoLanguage(), query: goSymbolQuery},
		".ts": {lang: grammars.TypescriptLanguage(), query: tsSymbolQuery},
		".tsx": {lang: grammars.TsxLanguage(), query: tsSymbolQuery},
		".js": {lang: grammars.JavascriptLanguage(), query: jsSymbolQuery},
		".jsx": {lang: grammars.JavascriptLanguage(), query: jsSymbolQuery},
		".py": {lang: grammars.PythonLanguage(), query: pySymbolQuery},
	}

	langs := make(map[string]*parsedLang, len(configs))
	for ext, cfg := range configs {
		q, err := gotreesitter.NewQuery(cfg.query, cfg.lang)
		if err != nil {
			return nil, fmt.Errorf("symbol query for %s: %w", ext, err)
		}
		langs[ext] = &parsedLang{lang: cfg.lang, query: q}
	}

	return &SymbolAwareChunker{
		fallback: fallback,
		logger:   logger.With().Str("component", "symbol-chunker").Logger(),
		langs:    langs,
	}, nil
}

func (s *SymbolAwareChunker) Chunk(content string, sourcePath string) []Chunk {
	ext := filepath.Ext(sourcePath)
	pl, ok := s.langs[ext]
	if !ok {
		return s.fallback.Chunk(content, sourcePath)
	}

	contentBytes := []byte(content)
	parser := gotreesitter.NewParser(pl.lang)
	tree, err := parser.Parse(contentBytes)
	if err != nil {
		s.logger.Warn().Err(err).Str("file", sourcePath).Msg("tree-sitter parse failed, using fallback")
		return s.fallback.Chunk(content, sourcePath)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	spans := s.extractSymbolSpans(bt, tree, pl, contentBytes, ext)
	if len(spans) == 0 {
		s.logger.Warn().Str("file", sourcePath).Msg("no symbols found, using fallback")
		return s.fallback.Chunk(content, sourcePath)
	}

	sort.Slice(spans, func(i, j int) bool {
		return spans[i].startByte < spans[j].startByte
	})

	lang := langForExt(ext)
	chunks := make([]Chunk, 0, len(spans))
	for i, sp := range spans {
		start := sp.startByte
		end := sp.endByte
		if int(end) > len(contentBytes) {
			end = uint32(len(contentBytes))
		}
		symbolContent := content[start:end]

		if len(symbolContent) > maxSymbolBytes {
			sub := s.fallback.Chunk(symbolContent, sourcePath)
			for j := range sub {
				sub[j].Sequence = len(chunks) + j
				sub[j].StartLine = lineForByteOffset(contentBytes, start)
				sub[j].EndLine = lineForByteOffset(contentBytes, end)
				sub[j].SymbolName = sp.name
				sub[j].SymbolKind = sp.kind
				sub[j].Language = lang
				sub[j].ChunkType = ChunkTypeSymbol
				sub[j].EmbeddingStrategy = EmbedStrategySymbol
			}
			chunks = append(chunks, sub...)
			continue
		}

		h := sha256.Sum256([]byte(symbolContent))
		chunks = append(chunks, Chunk{
			Content:           symbolContent,
			Sequence:          i,
			StartLine:         lineForByteOffset(contentBytes, start),
			EndLine:           lineForByteOffset(contentBytes, end),
			Hash:              fmt.Sprintf("%x", h),
			SymbolName:        sp.name,
			SymbolKind:        sp.kind,
			Language:          lang,
			ChunkType:         ChunkTypeSymbol,
			EmbeddingStrategy: EmbedStrategySymbol,
		})
	}

	for i := range chunks {
		chunks[i].Sequence = i
	}

	return chunks
}

func (s *SymbolAwareChunker) extractSymbolSpans(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, pl *parsedLang, content []byte, ext string) []symbolSpan {
	matches := pl.query.Execute(tree)
	var spans []symbolSpan
	seen := map[string]bool{}

	for _, match := range matches {
		var nameNode, declNode *gotreesitter.Node
		for _, cap := range match.Captures {
			switch cap.Name {
			case "name":
				nameNode = cap.Node
			case "decl":
				declNode = cap.Node
			}
		}
		if nameNode == nil || declNode == nil {
			continue
		}
		name := bt.NodeText(nameNode)
		if seen[name] {
			continue
		}
		seen[name] = true

		kind := inferKind(declNode, bt, ext)
		spans = append(spans, symbolSpan{
			name:      name,
			kind:      kind,
			startByte: declNode.StartByte(),
			endByte:   declNode.EndByte(),
		})
	}
	return spans
}

func inferKind(declNode *gotreesitter.Node, bt *gotreesitter.BoundTree, ext string) string {
	nodeType := bt.NodeType(declNode)
	switch nodeType {
	case "function_declaration", "function_definition":
		return "function"
	case "method_declaration", "method_definition":
		return "method"
	case "type_declaration", "type_alias_declaration":
		return "type"
	case "interface_declaration":
		return "interface"
	case "class_declaration", "class_definition":
		return "class"
	case "enum_declaration":
		return "enum"
	case "const_declaration":
		return "const"
	case "var_declaration", "lexical_declaration", "assignment":
		return "var"
	default:
		return "symbol"
	}
}

func langForExt(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	default:
		return ""
	}
}

func lineForByteOffset(content []byte, offset uint32) int {
	if int(offset) > len(content) {
		offset = uint32(len(content))
	}
	return bytes.Count(content[:offset], []byte("\n")) + 1
}

const goSymbolQuery = `
(function_declaration name: (identifier) @name) @decl
(method_declaration name: (field_identifier) @name) @decl
(type_declaration (type_spec name: (type_identifier) @name)) @decl
(const_declaration (const_spec name: (identifier) @name)) @decl
(var_declaration (var_spec name: (identifier) @name)) @decl
`

const tsSymbolQuery = `
(function_declaration name: (identifier) @name) @decl
(class_declaration name: (type_identifier) @name) @decl
(interface_declaration name: (type_identifier) @name) @decl
(type_alias_declaration name: (type_identifier) @name) @decl
(enum_declaration name: (identifier) @name) @decl
(lexical_declaration (variable_declarator name: (identifier) @name)) @decl
`

const jsSymbolQuery = `
(function_declaration name: (identifier) @name) @decl
(class_declaration name: (identifier) @name) @decl
(lexical_declaration (variable_declarator name: (identifier) @name)) @decl
`

const pySymbolQuery = `
(function_definition name: (identifier) @name) @decl
(class_definition name: (identifier) @name) @decl
`
