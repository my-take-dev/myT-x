package mcp

import (
	"context"
	"io"
)

// MCPRuntime abstracts the lifecycle of an MCP server runtime.
// Both lspmcp.Runtime and orchestrator.Runtime satisfy this interface.
type MCPRuntime interface {
	Start(ctx context.Context) error
	Serve(ctx context.Context) error
	Close(ctx context.Context) error
}

// RuntimeFactory creates an MCPRuntime for a single pipe connection.
// The returned runtime owns the provided reader/writer and serves MCP
// requests over them until closed.
type RuntimeFactory func(in io.Reader, out io.Writer) (MCPRuntime, error)
