Tracking: #131

## Why

Current `nano-brain init` asks 5 basic questions. Users need:
- Workspace directory prompt (for `init --root` after config)
- Ollama auto-detection (check if running on localhost before asking URL)
- Preview of generated config before writing
- Confirmation prompt before overwriting existing config

## What Changes

### Modified: `cmd/nano-brain/init.go`

Enhanced `runInteractiveInit`:
1. Add workspace directory question (default: current working directory)
2. Auto-detect Ollama on localhost:11434 before asking URL
3. Show config YAML preview before saving
4. Add Y/n confirmation before writing
5. After writing config: run doctor, then offer `init --root <workspace>` if server running
