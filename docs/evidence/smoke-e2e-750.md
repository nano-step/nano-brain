# smoke:e2e — Issue #750 (resolve accepts cwd)

Change-type: bug-fix. Ran on an isolated **:3199 / nanobrain_test** server built from this branch. Requests went over the real MCP streamable-HTTP transport (initialize, then tools/call).

curl -X POST http://localhost:3199/mcp  tools/call memory_workspaces_resolve {"cwd":"/data/workspaces/express-app"}
HTTP/1.1 200 OK
{"registered":false,"root_path":"/data/workspaces/express-app","workspace_hash":"2523829…","workspace_name":"express-app"}

curl -X POST http://localhost:3199/mcp  tools/call memory_workspaces_resolve {"path":"/data/workspaces/express-app"}
HTTP/1.1 200 OK
{"registered":false,"root_path":"/data/workspaces/express-app","workspace_hash":"2523829…","workspace_name":"express-app"}

Both calls return identical results. Before the fix, the `cwd` call returned `isError: "path is required"`, as seen 14 times in production proxy logs.

The server was killed by its captured PID.
