package summarize

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reBase64       = regexp.MustCompile(`[A-Za-z0-9+/]{100,}={0,2}`)
	reFencedBlock  = regexp.MustCompile("(?m)^(```)(\\w*)\\n([\\s\\S]*?)^```\\s*$")
	reErrorLine    = regexp.MustCompile(`(?i)^(error|ERROR|Error):`)
	reSystemHeader = regexp.MustCompile(`(?im)^(#{1,3})\s+system\b.*$`)
	reNextHeader   = regexp.MustCompile(`(?m)^(#{1,3})\s+`)

	reToolResultBlock = regexp.MustCompile(`(?m)^\*\*Tool result\*\*\s*\(([^)]*)\):\s*\n`)
	reToolOutputLabel = regexp.MustCompile(`(?im)^(tool_output:|(\*\*output:\*\*))\s*\n`)

	reClaudeToolUseCmd = regexp.MustCompile(`(?m)^\*\*tool_use\*\*\s+(\S+):\s*\n`)
)

// StripOpenCode reduces rendered OpenCode session markdown by removing
// system prompts, large tool outputs, large code blocks, repeated errors,
// and base64 data.
func StripOpenCode(content string) string {
	content = stripBase64(content)
	content = collapseCodeBlocks(content)
	content = replaceToolOutputs(content)
	content = dedupErrors(content)
	content = stripSystemSections(content)
	return content
}

// StripClaude reduces rendered Claude JSONL session markdown by replacing
// long tool_result outputs and long tool_use commands with compact placeholders.
func StripClaude(content string) string {
	content = stripBase64(content)
	content = replaceClaudeToolUseCommands(content)
	content = replaceClaudeToolResults(content)
	return content
}

func stripBase64(content string) string {
	return reBase64.ReplaceAllString(content, "[base64 data removed]")
}

func collapseCodeBlocks(content string) string {
	return reFencedBlock.ReplaceAllStringFunc(content, func(match string) string {
		subs := reFencedBlock.FindStringSubmatch(match)
		if len(subs) < 4 {
			return match
		}
		lang := subs[2]
		inner := subs[3]
		lineCount := countLines(inner)
		if lineCount <= 20 {
			return match
		}
		if lang == "" {
			lang = "unknown"
		}
		return fmt.Sprintf("[code block: %d lines, %s]", lineCount, lang)
	})
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

func stripSystemSections(content string) string {
	locs := reSystemHeader.FindAllStringIndex(content, -1)
	if len(locs) == 0 {
		return content
	}

	for i := len(locs) - 1; i >= 0; i-- {
		start := locs[i][0]
		headerMatch := content[locs[i][0]:locs[i][1]]
		headerLevel := countLeadingHashes(headerMatch)

		end := findNextHeaderOfEqualOrHigherLevel(content, locs[i][1], headerLevel)
		content = content[:start] + content[end:]
	}
	return content
}

func countLeadingHashes(line string) int {
	trimmed := strings.TrimLeft(line, " \t")
	n := 0
	for _, c := range trimmed {
		if c == '#' {
			n++
		} else {
			break
		}
	}
	return n
}

func findNextHeaderOfEqualOrHigherLevel(content string, searchFrom, level int) int {
	rest := content[searchFrom:]
	allHeaders := reNextHeader.FindAllStringIndex(rest, -1)
	for _, loc := range allHeaders {
		headerLine := rest[loc[0]:loc[1]]
		headerLevel := countLeadingHashes(headerLine)
		if headerLevel <= level {
			return searchFrom + loc[0]
		}
	}
	return len(content)
}

func replaceToolOutputs(content string) string {
	content = replaceToolResultBlocks(content)
	content = replaceToolOutputLabeled(content)
	return content
}

func replaceToolResultBlocks(content string) string {
	locs := reToolResultBlock.FindAllStringSubmatchIndex(content, -1)
	if len(locs) == 0 {
		return content
	}

	for i := len(locs) - 1; i >= 0; i-- {
		fullMatchEnd := locs[i][1]
		nameStart, nameEnd := locs[i][2], locs[i][3]
		name := "unknown"
		if nameStart >= 0 && nameEnd >= 0 {
			name = strings.TrimSpace(content[nameStart:nameEnd])
		}
		if name == "" {
			name = "unknown"
		}

		body, bodyEnd := extractBody(content, fullMatchEnd)
		if len(body) <= 200 {
			continue
		}
		bodyLines := countLines(body)
		replacement := fmt.Sprintf("**Tool result** (%s):\n[tool: %s, %d lines]\n", name, name, bodyLines)
		content = content[:locs[i][0]] + replacement + content[bodyEnd:]
	}
	return content
}

func replaceToolOutputLabeled(content string) string {
	locs := reToolOutputLabel.FindAllStringIndex(content, -1)
	if len(locs) == 0 {
		return content
	}

	for i := len(locs) - 1; i >= 0; i-- {
		body, bodyEnd := extractFencedOrIndentedBody(content, locs[i][1])
		if len(body) <= 200 {
			continue
		}
		bodyLines := countLines(body)
		replacement := fmt.Sprintf("[tool: unknown, %d lines]\n", bodyLines)
		content = content[:locs[i][1]] + replacement + content[bodyEnd:]
	}
	return content
}

func extractBody(content string, from int) (string, int) {
	if from >= len(content) {
		return "", from
	}

	if strings.HasPrefix(content[from:], "```") {
		return extractFencedBody(content, from)
	}
	return extractUntilBreak(content, from)
}

func extractFencedBody(content string, from int) (string, int) {
	endFence := strings.Index(content[from:], "\n```")
	if endFence < 0 {
		return content[from:], len(content)
	}
	closingEnd := from + endFence
	for closingEnd < len(content) && content[closingEnd] != '\n' {
		closingEnd++
	}
	closingEnd++
	endOfClosingFence := closingEnd
	for endOfClosingFence < len(content) {
		if content[endOfClosingFence] == '\n' {
			endOfClosingFence++
			break
		}
		endOfClosingFence++
	}
	return content[from:endOfClosingFence], endOfClosingFence
}

func extractUntilBreak(content string, from int) (string, int) {
	lines := strings.SplitAfter(content[from:], "\n")
	var body strings.Builder
	pos := from
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "**") && body.Len() > 0 {
			break
		}
		if trimmed == "" && body.Len() > 0 {
			remaining := content[pos+len(line):]
			if len(remaining) > 0 {
				nextLine := strings.SplitN(remaining, "\n", 2)[0]
				nextTrimmed := strings.TrimSpace(nextLine)
				if nextTrimmed != "" && !strings.HasPrefix(nextLine, "    ") && !strings.HasPrefix(nextLine, "\t") {
					body.WriteString(line)
					pos += len(line)
					break
				}
			}
		}
		body.WriteString(line)
		pos += len(line)
	}
	return body.String(), pos
}

func extractFencedOrIndentedBody(content string, from int) (string, int) {
	if from >= len(content) {
		return "", from
	}
	if strings.HasPrefix(content[from:], "```") {
		return extractFencedBody(content, from)
	}
	return extractUntilBreak(content, from)
}

func dedupErrors(content string) string {
	lines := strings.Split(content, "\n")
	errorCounts := make(map[string]int)
	errorFirstIdx := make(map[string]int)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if reErrorLine.MatchString(trimmed) {
			errorCounts[trimmed]++
			if errorCounts[trimmed] == 1 {
				errorFirstIdx[trimmed] = i
			}
		}
	}

	toRemove := make(map[int]bool)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !reErrorLine.MatchString(trimmed) {
			continue
		}
		count := errorCounts[trimmed]
		firstIdx := errorFirstIdx[trimmed]
		if count <= 1 {
			continue
		}
		if i == firstIdx {
			lines[i] = line + fmt.Sprintf(" (repeated %d more times)", count-1)
		} else {
			toRemove[i] = true
		}
	}

	if len(toRemove) == 0 {
		return content
	}

	var result []string
	for i, line := range lines {
		if !toRemove[i] {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func replaceClaudeToolUseCommands(content string) string {
	locs := reClaudeToolUseCmd.FindAllStringSubmatchIndex(content, -1)
	if len(locs) == 0 {
		return content
	}

	for i := len(locs) - 1; i >= 0; i-- {
		fullMatchEnd := locs[i][1]
		nameStart, nameEnd := locs[i][2], locs[i][3]
		name := "unknown"
		if nameStart >= 0 && nameEnd >= 0 {
			name = strings.TrimSpace(content[nameStart:nameEnd])
		}

		body, bodyEnd := extractFencedOrIndentedBody(content, fullMatchEnd)
		bodyLineCount := countLines(body)
		if bodyLineCount <= 5 {
			continue
		}
		firstLine := strings.SplitN(body, "\n", 2)[0]
		if len(firstLine) > 80 {
			firstLine = firstLine[:80]
		}
		replacement := fmt.Sprintf("**tool_use** %s:\n[command: %s...]\n", name, firstLine)
		content = content[:locs[i][0]] + replacement + content[bodyEnd:]
	}
	return content
}

func replaceClaudeToolResults(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		if !strings.HasPrefix(line, "## tool_result") {
			result = append(result, line)
			i++
			continue
		}

		result = append(result, line)
		i++

		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			result = append(result, lines[i])
			i++
		}

		var body []string
		bodyStart := i
		for i < len(lines) {
			if i > bodyStart && strings.HasPrefix(lines[i], "## ") {
				break
			}
			body = append(body, lines[i])
			i++
		}

		bodyText := strings.Join(body, "\n")
		if len(bodyText) > 200 {
			bodyLines := len(body)
			result = append(result, fmt.Sprintf("[tool: tool_result, %d lines]", bodyLines))
		} else {
			result = append(result, body...)
		}
	}
	return strings.Join(result, "\n")
}
