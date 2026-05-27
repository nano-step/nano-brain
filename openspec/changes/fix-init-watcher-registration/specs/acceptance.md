## AC-1: watcher picks up new workspace without restart

**Given** the server is running with no workspaces registered  
**When** `POST /api/v1/init` is called with a valid `root_path`  
**Then** the watcher immediately starts watching the `code` collection path  
**And** a server restart is NOT required for the watcher to activate

## AC-2: nil watcher is safe

**Given** `InitWorkspace` is called with `fw = nil` (test scenario)  
**When** the handler completes  
**Then** no panic occurs and the workspace is registered normally in the DB

## AC-3: watcher errors are non-fatal

**Given** `fw.WatchWithFilter` returns an error (e.g. path does not exist)  
**When** `InitWorkspace` is called  
**Then** the handler still returns HTTP 200 with the workspace hash  
**And** the watcher error is logged as WARN, not returned as HTTP error

## AC-4: existing tests pass

**Given** the test suite calls `InitWorkspace(q, nil, nil, config.WatcherConfig{}, logger)`  
**When** tests run  
**Then** all pass with exit 0
