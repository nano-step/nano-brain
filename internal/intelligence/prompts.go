package intelligence

const consolidationSystemPrompt = `You are a memory consolidation assistant. Your task is to analyze sets of similar documents and determine if they should be merged.

Given multiple documents that appear semantically similar, you must:
1. Determine if they contain overlapping or redundant information
2. If they do overlap, create a single consolidated document that preserves ALL unique information from each source
3. If they are actually distinct (just similar topics), indicate they should remain separate

Return your response as JSON with this structure:
{
  "should_merge": true/false,
  "reasoning": "brief explanation of your decision",
  "consolidated_content": "the merged markdown content (only if should_merge is true)",
  "title": "appropriate title for the consolidated document (only if should_merge is true)"
}

IMPORTANT:
- If should_merge is false, omit consolidated_content and title fields
- When merging, preserve all unique facts, decisions, and context
- Use clear markdown formatting in consolidated_content
- The consolidated document should be comprehensive but not verbose`

const categorizationSystemPrompt = `You are a document categorization assistant. Your task is to analyze documents and assign appropriate semantic tags.

Available tags:
- bug-fix: fixes for bugs or issues
- feature: new functionality or capabilities
- refactor: code restructuring without behavior change
- docs: documentation updates
- chore: maintenance tasks, dependency updates
- architecture: system design decisions
- debugging: investigation and diagnosis
- research: exploration and analysis
- decision: architectural or technical decisions

Analyze the document and return JSON:
{
  "tags": ["tag1", "tag2", ...],
  "confidence": 0.0-1.0,
  "reasoning": "brief explanation"
}

Guidelines:
- Assign 1-3 most relevant tags
- Be conservative: only assign tags you're confident about
- Confidence should reflect how clear the categorization is
- Tags should be from the predefined list above`

func buildConsolidationUserPrompt(docs []DocumentSummary) string {
	prompt := "Please analyze these documents and determine if they should be consolidated:\n\n"
	
	for i, doc := range docs {
		prompt += "---\n"
		prompt += "Document " + string(rune('A'+i)) + ":\n"
		prompt += "Title: " + doc.Title + "\n"
		prompt += "Source: " + doc.SourcePath + "\n"
		prompt += "Tags: " + formatTags(doc.Tags) + "\n"
		prompt += "Content (first 1500 chars):\n"
		
		content := doc.Content
		if len(content) > 1500 {
			content = content[:1500] + "... (truncated)"
		}
		prompt += content + "\n\n"
	}
	
	return prompt
}

func buildCategorizationUserPrompt(doc DocumentSummary) string {
	prompt := "Please categorize this document:\n\n"
	prompt += "Title: " + doc.Title + "\n"
	prompt += "Source: " + doc.SourcePath + "\n"
	prompt += "Current tags: " + formatTags(doc.Tags) + "\n\n"
	prompt += "Content (first 2000 chars):\n"
	
	content := doc.Content
	if len(content) > 2000 {
		content = content[:2000] + "... (truncated)"
	}
	prompt += content
	
	return prompt
}

func formatTags(tags []string) string {
	if len(tags) == 0 {
		return "(none)"
	}
	result := ""
	for i, tag := range tags {
		if i > 0 {
			result += ", "
		}
		result += tag
	}
	return result
}

type DocumentSummary struct {
	ID         string
	Title      string
	SourcePath string
	Content    string
	Tags       []string
}
