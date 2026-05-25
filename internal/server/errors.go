package server

import "errors"

// ErrWorkspaceRequired is returned when a workspace is required but not provided.
var ErrWorkspaceRequired = errors.New("workspace is required")

// ErrUnsupportedMediaType is returned when the request Content-Type is not application/json.
var ErrUnsupportedMediaType = errors.New("Content-Type must be application/json")
