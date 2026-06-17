package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

func registerMemoryFlowchart(server *mcpsdk.Server, a *Adapter) {
	server.AddTool(
		&mcpsdk.Tool{
			Name:        "memory_flowchart",
			Description: "Return the control-flow graph (flowchart) for a specific function, identified by 'file::startLine-endLine' (e.g. 'src/routes/purchase.ts::15-48'). The CFG has decision/step/terminal nodes and labeled branch edges. Returns found:false when no flowchart is stored for that span (e.g. a non-JS/TS function, since CFG extraction is JS/TS only) or when flow indexing is disabled.",
			InputSchema: toolSchema(map[string]map[string]any{
				"workspace": {"type": "string", "description": "Workspace identifier — name (e.g. 'nano-brain') or full hash"},
				"node":      {"type": "string", "description": "Function span as 'file::startLine-endLine', e.g. 'src/routes/purchase.ts::15-48'"},
			}, []string{"workspace", "node"}),
		},
		func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			if !a.flowCfg.Enabled {
				return textResult(map[string]any{
					"found":   false,
					"message": "flow indexing disabled",
				})
			}

			args, err := parseArgs(req.Params.Arguments)
			if err != nil {
				return errResult("invalid arguments"), nil
			}
			ws, errRes := a.requireWorkspace(ctx, args)
			if errRes != nil {
				return errRes, nil
			}
			node := argString(args, "node")
			if node == "" {
				return errResult("node is required"), nil
			}

			file, startLine, endLine, perr := parseFlowchartNode(node)
			if perr != nil {
				return errResult(perr.Error()), nil
			}

			fc, err := a.queries.GetFunctionFlowchart(ctx, sqlc.GetFunctionFlowchartParams{
				WorkspaceHash: ws,
				SourceFile:    file,
				StartLine:     int32(startLine),
				EndLine:       int32(endLine),
			})
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return textResult(map[string]any{
						"found": false,
						"node":  node,
					})
				}
				return errResult(fmt.Sprintf("flowchart query failed: %v", err)), nil
			}

			return textResult(map[string]any{
				"found":  true,
				"entry":  fc.Entry,
				"status": fc.Status,
				"cfg":    json.RawMessage(fc.Cfg),
			})
		},
	)
}

// parseFlowchartNode parses a "file::startLine-endLine" identifier.
func parseFlowchartNode(node string) (file string, startLine, endLine int, err error) {
	parts := strings.SplitN(node, "::", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", 0, 0, errors.New("node must be 'file::startLine-endLine'")
	}
	file = parts[0]
	lineRange := strings.SplitN(parts[1], "-", 2)
	if len(lineRange) != 2 {
		return "", 0, 0, errors.New("node line range must be 'startLine-endLine'")
	}
	startLine, err = strconv.Atoi(strings.TrimSpace(lineRange[0]))
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid start line: %v", err)
	}
	endLine, err = strconv.Atoi(strings.TrimSpace(lineRange[1]))
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid end line: %v", err)
	}
	return file, startLine, endLine, nil
}
