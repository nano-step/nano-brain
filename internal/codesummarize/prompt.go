package codesummarize

import (
	"fmt"
	"strings"
)

const promptTemplate = `You are a code documentation assistant. Summarize the purpose and behavior of each code symbol below.

For each symbol, provide a concise 1-3 sentence summary that explains:
- What the function/type/constant does
- Key parameters or fields (if complex)
- Return value or purpose (if not obvious from name)

Output ONLY a JSON object containing a "summaries" key with an array value, using this exact structure:
{
  "summaries": [
    {"name": "symbolName", "file": "path/to/file.go", "summary": "Your summary here"},
    ...
  ]
}

Do not include any other text, explanations, or markdown - just the JSON object.

`

func BuildBatchPrompt(symbols []SymbolForSummary) string {
	return BuildBatchPromptWithContext(symbols, nil)
}

func BuildBatchPromptWithContext(symbols []SymbolForSummary, graphContexts map[string]*SymbolGraphContext) string {
	var b strings.Builder
	b.WriteString(promptTemplate)

	b.WriteString("Symbols to summarize:\n\n")

	for i, sym := range symbols {
		fmt.Fprintf(&b, "### %d. %s (in %s)\n", i+1, sym.Name, sym.File)
		fmt.Fprintf(&b, "Kind: %s\n", sym.Kind)
		if sym.Language != "" {
			fmt.Fprintf(&b, "Language: %s\n", sym.Language)
		}

		if graphContexts != nil {
			nodeKey := sym.File + "::" + sym.Name
			if gc, ok := graphContexts[nodeKey]; ok {
				if ctx := FormatGraphContextForPrompt(gc); ctx != "" {
					fmt.Fprintf(&b, "Context:\n%s", ctx)
				}
			}
		}

		fmt.Fprintf(&b, "\nCode:\n```\n%s\n```\n\n", sym.Code)
	}

	return b.String()
}
