## Design

### Modified file: `cmd/nano-brain/init.go`

Current: 131 lines. Target: ~180 lines (add preview, confirmation, auto-detect, workspace prompt).

### New flow:

```
1. Check existing config → overwrite confirmation
2. Prompt: PostgreSQL URL
3. Prompt: Embedding provider (ollama/voyage)
4. Auto-detect Ollama if provider=ollama
5. Prompt: Ollama URL / Voyage model+key
6. Prompt: Embedding model
7. Prompt: Server port
8. Show YAML preview
9. Confirm save → write file
10. Run doctor
11. Ask: register workspace? → hint or call API
```

### Auto-detection

```go
func detectOllama(url string) bool {
    client := &http.Client{Timeout: 2 * time.Second}
    resp, err := client.Get(url)
    if err != nil { return false }
    resp.Body.Close()
    return resp.StatusCode == 200
}
```

### Dependencies
- Add `net/http` and `time` imports to init.go
- No new external dependencies
