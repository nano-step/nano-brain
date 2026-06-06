package codesummarize

const (
	promptOverheadTokens  = 150
	perSymbolHeaderTokens = 25
	charsPerToken         = 4
)

// EstimateTokens approximates the token count for a batch of symbols.
// Uses heuristic: 150 tokens prompt overhead + 25 tokens per symbol header + code length / 4.
func EstimateTokens(symbols []SymbolForSummary) int {
	tokens := promptOverheadTokens
	for _, sym := range symbols {
		tokens += perSymbolHeaderTokens + len(sym.Code)/charsPerToken
	}
	return tokens
}
