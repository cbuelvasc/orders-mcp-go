package domain

import "errors"

// Sentinel errors. Adapters map them to their protocol (HTTP status, MCP tool error, ...).
var (
	ErrInvalidInput      = errors.New("invalid input")
	ErrOrderNotFound     = errors.New("order not found")
	ErrCannotBeCancelled = errors.New("order cannot be cancelled")
)
