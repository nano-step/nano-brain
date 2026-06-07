package codesummarize

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type CallerInfo struct {
	Name      string
	Frequency int
}

type SymbolGraphContext struct {
	Callers []CallerInfo
	Callees []string
}

type GraphContextQuerier interface {
	BulkGetCallerContext(ctx context.Context, arg sqlc.BulkGetCallerContextParams) ([]sqlc.BulkGetCallerContextRow, error)
	BulkGetCalleeNodes(ctx context.Context, arg sqlc.BulkGetCalleeNodesParams) ([]sqlc.BulkGetCalleeNodesRow, error)
}

const maxCallersInPrompt = 10

func FetchGraphContext(ctx context.Context, q GraphContextQuerier, workspaceHash string, symbolNodes []string) (map[string]*SymbolGraphContext, error) {
	if len(symbolNodes) == 0 {
		return nil, nil
	}

	result := make(map[string]*SymbolGraphContext, len(symbolNodes))

	callerRows, err := q.BulkGetCallerContext(ctx, sqlc.BulkGetCallerContextParams{
		WorkspaceHash: workspaceHash,
		Column2:       symbolNodes,
	})
	if err != nil {
		return nil, fmt.Errorf("bulk get caller context: %w", err)
	}

	for _, row := range callerRows {
		gc, ok := result[row.TargetNode]
		if !ok {
			gc = &SymbolGraphContext{}
			result[row.TargetNode] = gc
		}
		gc.Callers = append(gc.Callers, CallerInfo{
			Name:      row.SourceNode,
			Frequency: int(row.Frequency),
		})
	}

	calleeRows, err := q.BulkGetCalleeNodes(ctx, sqlc.BulkGetCalleeNodesParams{
		WorkspaceHash: workspaceHash,
		Column2:       symbolNodes,
	})
	if err != nil {
		return nil, fmt.Errorf("bulk get callee nodes: %w", err)
	}

	for _, row := range calleeRows {
		gc, ok := result[row.SourceNode]
		if !ok {
			gc = &SymbolGraphContext{}
			result[row.SourceNode] = gc
		}
		gc.Callees = append(gc.Callees, row.TargetNode)
	}

	return result, nil
}

func ComputeGraphContextHash(gc *SymbolGraphContext) string {
	if gc == nil || (len(gc.Callers) == 0 && len(gc.Callees) == 0) {
		return ""
	}

	callerNames := make([]string, len(gc.Callers))
	for i, c := range gc.Callers {
		callerNames[i] = c.Name
	}
	sort.Strings(callerNames)

	callees := make([]string, len(gc.Callees))
	copy(callees, gc.Callees)
	sort.Strings(callees)

	h := sha256.New()
	h.Write([]byte(strings.Join(callerNames, ",")))
	h.Write([]byte("|"))
	h.Write([]byte(strings.Join(callees, ",")))
	return hex.EncodeToString(h.Sum(nil))
}

func FormatGraphContextForPrompt(gc *SymbolGraphContext) string {
	if gc == nil || (len(gc.Callers) == 0 && len(gc.Callees) == 0) {
		return ""
	}

	var b strings.Builder

	if len(gc.Callers) > 0 {
		b.WriteString("- TRIGGERED BY: ")
		limit := len(gc.Callers)
		overflow := 0
		if limit > maxCallersInPrompt {
			overflow = limit - maxCallersInPrompt
			limit = maxCallersInPrompt
		}
		for i := 0; i < limit; i++ {
			if i > 0 {
				b.WriteString(", ")
			}
			c := gc.Callers[i]
			shortName := lastSegment(c.Name)
			fmt.Fprintf(&b, "%s (%d calls)", shortName, c.Frequency)
		}
		if overflow > 0 {
			fmt.Fprintf(&b, ", and %d more callers", overflow)
		}
		b.WriteString("\n")
	}

	if len(gc.Callees) > 0 {
		b.WriteString("- CALLS: ")
		for i, callee := range gc.Callees {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(lastSegment(callee))
		}
		b.WriteString("\n")
	}

	text := b.String()
	if len(text) > 4000 {
		text = text[:4000]
	}
	return text
}

func lastSegment(node string) string {
	if idx := strings.LastIndex(node, "::"); idx >= 0 {
		return node[idx+2:]
	}
	if idx := strings.LastIndex(node, "/"); idx >= 0 {
		return node[idx+1:]
	}
	return node
}
