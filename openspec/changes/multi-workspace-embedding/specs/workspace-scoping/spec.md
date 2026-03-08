## MODIFIED Requirements

### Requirement: Workspace detection from PWD
The MCP server SHALL compute a `projectHash` from `process.cwd()` at startup using `sha256(cwd).substring(0, 12)`. This hash SHALL be stored as `currentProjectHash` on the server context and used for all default search filtering. Additionally, the server SHALL be able to compute project hashes for any workspace path listed in `config.yml`, not just the startup directory.

#### Scenario: Server starts in a workspace directory
- **WHEN** the MCP server starts with `PWD=/Users/alice/projects/my-app`
- **THEN** `currentProjectHash` is set to the first 12 characters of `sha256("/Users/alice/projects/my-app")`
- **THEN** the hash is consistent across restarts in the same directory

#### Scenario: Hash matches harvester convention
- **WHEN** the MCP server computes `currentProjectHash` for a workspace
- **THEN** the hash matches the directory name used by the harvester for that workspace's sessions (`sessions/{projectHash}/*.md`)

#### Scenario: Compute hash for non-startup workspace
- **WHEN** the server needs to process a workspace from config.yml that is not the startup directory
- **THEN** it computes the project hash using the same `sha256(path).substring(0, 12)` convention
- **THEN** it resolves the DB filename as `{dirName}-{hash}.sqlite`
