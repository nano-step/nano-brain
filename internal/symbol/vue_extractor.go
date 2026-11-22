package symbol

import (
	"fmt"

	"github.com/nano-brain/nano-brain/internal/tsparse"
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
	vueLang *gotreesitter.Language
	tsLang  *gotreesitter.Language
	jsLang  *gotreesitter.Language
	tsQ     *gotreesitter.Query
	jsQ     *gotreesitter.Query
}

func NewVueExtractor() (*VueExtractor, error) {
	vueLang := grammars.VueLanguage()
	tsLang := grammars.TypescriptLanguage()
	jsLang := grammars.JavascriptLanguage()

	tsQ, err := gotreesitter.NewQuery(tsQuery, tsLang)
	if err != nil {
		return nil, fmt.Errorf("vue ts symbol query: %w", err)
	}
	jsQ, err := gotreesitter.NewQuery(jsQuery, jsLang)
	if err != nil {
		return nil, fmt.Errorf("vue js symbol query: %w", err)
	}

	ex := &VueExtractor{
		vueLang: vueLang,
		tsLang:  tsLang,
		jsLang:  jsLang,
		tsQ:     tsQ,
		jsQ:     jsQ,
	}
	if _, err := ex.newInjectionParser(); err != nil {
		return nil, err
	}
	return ex, nil
}

func (e *VueExtractor) newInjectionParser() (*gotreesitter.InjectionParser, error) {
	ip := tsparse.NewInjectionParser()
	ip.RegisterLanguage("vue", e.vueLang)
	ip.RegisterLanguage("typescript", e.tsLang)
	ip.RegisterLanguage("javascript", e.jsLang)
	if err := ip.RegisterInjectionQuery("vue", vueSymbolInjectionQuery); err != nil {
		return nil, fmt.Errorf("vue symbol injection query: %w", err)
	}
	return ip, nil
}

func (e *VueExtractor) Supports(ext string) bool {
	return ext == ".vue"
}

func (e *VueExtractor) Extract(filePath string, content []byte) ([]Symbol, error) {
	ip, err := e.newInjectionParser()
	if err != nil {
		return nil, err
	}
	result, err := ip.Parse(content, "vue")
	if err != nil {
		return nil, fmt.Errorf("parse vue sfc %s: %w", filePath, err)
	}
	if result == nil {
		return nil, nil
	}
	defer func() {
		if result.Tree != nil {
			result.Tree.Release()
		}
		for _, inj := range result.Injections {
			if inj.Tree != nil {
				inj.Tree.Release()
			}
		}
	}()

	var symbols []Symbol
	seen := map[string]rankedSymbolIndex{}

	for _, inj := range result.Injections {
		if inj.Tree == nil {
			continue
		}
		var q *gotreesitter.Query
		var lang *gotreesitter.Language
		switch inj.Language {
		case "typescript":
			q = e.tsQ
			lang = e.tsLang
		case "javascript":
			q = e.jsQ
			lang = e.jsLang
		default:
			continue
		}

		bt := gotreesitter.Bind(inj.Tree)
		matches := q.Execute(inj.Tree)
		appendJSTSSymbols(&symbols, seen, bt, lang, matches, content, filePath, "vue")
		bt.Release()
	}

	return symbols, nil
}
