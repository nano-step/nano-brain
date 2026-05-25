# Spec: Server Logging

## ADDED Requirements

### Requirement: HTTP request logging middleware
Every HTTP request MUST be logged at completion with at minimum: method, path, status code, latency.

#### Scenario: Successful request
- Given a client sends `POST /api/v1/write`
- When the handler returns HTTP 200
- Then the log file contains an entry at info level with fields: `method`, `path`, `status` (200), `latency_ms`, `request_id`

#### Scenario: Failed request (4xx)
- Given a client sends a request with missing required fields
- When the handler returns HTTP 400
- Then the log file contains an entry with `status` 400

#### Scenario: Server error (5xx)
- Given a handler returns HTTP 500
- Then the log file contains an entry at error level with `status` 500

#### Scenario: Request ID propagation
- Given a client sends a request without `X-Request-ID` header
- Then the middleware generates a unique `request_id` and includes it in the log entry
- And the response includes `X-Request-ID` header with the same value

#### Scenario: Client-provided request ID
- Given a client sends `X-Request-ID: my-custom-id`
- Then `request_id` in the log entry equals `my-custom-id`

### Requirement: Handler success-path INFO logs
Mutating handlers MUST log at INFO level on successful completion.

#### Scenario: Workspace registration logged
- Given `POST /api/v1/init` succeeds
- Then the log contains an entry with `"message":"workspace registered"` and `workspace_hash` field

#### Scenario: Document write logged
- Given `POST /api/v1/write` succeeds
- Then the log contains an entry with `"message":"document written"` and `document_id` field

#### Scenario: Collection add logged
- Given `POST /api/v1/collections` succeeds
- Then the log contains an entry with `"message":"collection added"` and `name` field

#### Scenario: Reindex queued logged
- Given `POST /api/v1/reindex` accepts (202)
- Then the log contains an entry with `"message":"reindex queued"` and `workspace` field

### Requirement: Middleware ordering
The request-logging middleware MUST be registered before workspace middleware so every request (including those rejected by workspace middleware) is logged.

#### Scenario: Request rejected by workspace middleware
- Given a request arrives without a workspace identifier
- When workspace middleware rejects it with 400
- Then the request-logging middleware still records the completed request at info level with status 400
