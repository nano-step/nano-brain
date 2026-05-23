package server

import "errors"

// ErrWorkspaceRequired is returned when a workspace is required but not provided.
var ErrWorkspaceRequired = errors.New("workspace is required")
