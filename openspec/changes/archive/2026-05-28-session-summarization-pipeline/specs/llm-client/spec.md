## ADDED Requirements

### Requirement: OpenAI-compatible chat completion client
The system SHALL provide an HTTP client that sends chat completion requests to any OpenAI-compatible API endpoint (`/v1/chat/completions`). The client SHALL use `net/http` with no external SDK dependencies.

#### Scenario: Successful completion request
- **WHEN** the client sends a chat completion request with system + user messages
- **THEN** the client SHALL return the assistant's response text and token usage counts

#### Scenario: SSE streaming response parsing
- **WHEN** the provider returns a `text/event-stream` response (SSE)
- **THEN** the client SHALL parse `data:` lines, concatenate `choices[0].delta.content` fields, and return the full response text after receiving `data: [DONE]`

#### Scenario: Non-streaming response
- **WHEN** the provider returns a JSON response (non-streaming)
- **THEN** the client SHALL parse `choices[0].message.content` and return it directly

### Requirement: Configurable LLM provider
The system SHALL read LLM provider configuration from the `summarization` section of config.yaml. The configuration SHALL include: `enabled` (bool), `provider_url` (string), `api_key` (string), `model` (string), `max_tokens` (int), `concurrency` (int), `output_dir` (string).

#### Scenario: Config with ai-proxy provider
- **WHEN** config contains `provider_url: "https://ai-proxy.thnkandgrow.com/v1"` and `model: "nano-brain"`
- **THEN** the client SHALL send requests to `https://ai-proxy.thnkandgrow.com/v1/chat/completions` with `Authorization: Bearer {api_key}` header

#### Scenario: Config with environment variable API key
- **WHEN** `api_key` is empty in config but `NANO_BRAIN_SUMMARIZE_API_KEY` environment variable is set
- **THEN** the client SHALL use the environment variable value

#### Scenario: Summarization disabled
- **WHEN** `enabled: false` in config
- **THEN** the harvest pipeline SHALL skip summarization entirely and log an info message

### Requirement: Retry with backoff on transient errors
The client SHALL retry failed requests up to 3 times with exponential backoff (1s, 2s, 4s) for HTTP 429 (rate limit) and 5xx (server error) status codes.

#### Scenario: Rate limited by provider
- **WHEN** the provider returns HTTP 429
- **THEN** the client SHALL wait and retry up to 3 times before returning an error

#### Scenario: Non-retryable error
- **WHEN** the provider returns HTTP 400 or 401
- **THEN** the client SHALL return the error immediately without retry

### Requirement: Structured logging for all LLM calls
Every LLM request SHALL be logged with: model, token counts (input/output), latency, success/failure status.

#### Scenario: Successful LLM call logging
- **WHEN** a chat completion succeeds
- **THEN** the system SHALL log at INFO level with fields: model, prompt_tokens, completion_tokens, latency_ms

#### Scenario: Failed LLM call logging
- **WHEN** a chat completion fails after retries
- **THEN** the system SHALL log at WARN level with fields: model, error, attempts, latency_ms
