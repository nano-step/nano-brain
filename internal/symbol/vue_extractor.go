package symbol

import (
	"fmt"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const vueSymbolInjectionQuery = `
(script_element
  (raw_text) @injection.content
)
(#set! injection.language "typescript")
`

type VueExtractor struct {
	ip     *gotreesitter.InjectionParser
	tsLang *gotreesitter.Language
	jsLang *gotreesitter.Language
	tsQ    *gotreesitter.Query
	jsQ    *gotreesitter.Query
}

func NewVueExtractor() (*VueExtractor, error) {
	vueLang := grammars.VueLanguage()
	tsLang := grammars.TypescriptLanguage()
	jsLang := grammars.JavascriptLanguage()

	ip := gotreesitter.NewInjectionParser()
	ip.RegisterLanguage("vue", vueLang)
	ip.RegisterLanguage("typescript", tsLang)
	ip.RegisterLanguage("javascript", jsLang)
	if err := ip.RegisterInjectionQuery("vue", vueSymbolInjectionQuery); err != nil {
		return nil, fmt.Errorf("vue symbol injection query: %w", err)
	}

	tsQ, err := gotreesitter.NewQuery(tsQuery, tsLang)
	if err != nil {
		return nil, fmt.Errorf("vue ts symbol query: %w", err)
	}
	jsQ, err := gotreesitter.NewQuery(jsQuery, jsLang)
	if err != nil {
		return nil, fmt.Errorf("vue js symbol query: %w", err)
	}

	return &VueExtractor{
		ip:     ip,
		tsLang: tsLang,
		jsLang: jsLang,
		tsQ:    tsQ,
		jsQ:    jsQ,
	}, nil
}

func (e *VueExtractor) Supports(ext string) bool {
	return ext == ".vue"
}

func (e *VueExtractor) Extract(filePath string, content []byte) ([]Symbol, error) {
	result, err := e.ip.Parse(content, "vue")
	if err != nil {
		return nil, fmt.Errorf("parse vue sfc %s: %w", filePath, err)
	}
	defer func() {
		if result != nil && result.Tree != nil {
			result.Tree.Release()
		}
	}()

	var symbols []Symbol
	seen := map[string]bool{}

	for _, inj := range result.Injections {
		if inj.Tree == nil {
			continue
		}
		var q *gotreesitter.Query
		var isTS bool
		switch inj.Language {
		case "typescript":
			q = e.tsQ
			isTS = true
		case "javascript":
			q = e.jsQ
		default:
			continue
		}

		bt := gotreesitter.Bind(inj.Tree)
		matches := q.Execute(inj.Tree)
		for _, match := range matches {
			var declNode, nameNode *gotreesitter.Node
			for _, cap := range match.Captures {
				switch cap.Name {
				case "decl":
					declNode = cap.Node
				case "name":
					nameNode = cap.Node
				}
			}
			if nameNode == nil || declNode == nil {
				continue
			}
			name := bt.NodeText(nameNode)
			var kind Kind
			if isTS {
				kind = tsNodeKind(bt, declNode)
			} else {
				kind = jsNodeKind(bt, declNode)
			}
			key := string(kind) + ":" + name
			if seen[key] {
				continue
			}
			seen[key] = true
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      kind,
				File:      filePath,
				Line:      lineForByte(content, nameNode.StartByte()),
				EndLine:   lineForByte(content, declNode.EndByte()),
				Signature: firstLine(bt.NodeText(declNode)),
				Language:  "vue",
			})
		}
		bt.Release()
	}

	return symbols, nil
}
