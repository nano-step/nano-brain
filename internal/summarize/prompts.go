package summarize

import (
	"fmt"
	"strings"
)

const MapSystemPrompt = `You are summarizing one chunk of an AI coding session.
Extract ONLY what is in this chunk. Be concise. Use bullet points. Format:

ACTIVITIES: <what happened>
DECISIONS: <choices made, with rationale if stated>
FILES: <paths mentioned>
PROBLEMS: <errors, bugs, blockers encountered>
LEARNINGS: <insights, patterns, conclusions>

If a section has nothing, write "(none)". Do not invent details not in the chunk.`

const ReduceSystemPrompt = `You are merging chunk summaries from an AI coding session into one final summary.
Deduplicate, organize chronologically where it matters, and output markdown with exactly these 5 sections (in this order):

## Goal
One paragraph stating the overall objective of the session.

## Decisions Made
Bullet list of concrete decisions with rationale.

## Files Touched
Bullet list of file paths mentioned, grouped where related.

## Problems Encountered
Bullet list of errors / blockers, with resolution status if known.

## Key Learnings
Bullet list of insights worth remembering across sessions.

Be concise. Do not repeat the chunk summaries verbatim — synthesize.`

const SingleShotSystemPrompt = ReduceSystemPrompt

func FormatMapUserPrompt(chunkContent string) string {
	return "Summarize this session chunk:\n\n" + chunkContent
}

func FormatReduceUserPrompt(chunkSummaries []string) string {
	var b strings.Builder
	for i, s := range chunkSummaries {
		fmt.Fprintf(&b, "Chunk %d:\n%s\n\n", i+1, s)
	}
	return "Merge these chunk summaries into a single session summary:\n\n" + b.String()
}

func FormatSingleShotUserPrompt(content string) string {
	return "Summarize this AI coding session into the 5-section markdown format:\n\n" + content
}
