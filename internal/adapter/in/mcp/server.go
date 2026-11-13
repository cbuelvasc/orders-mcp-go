package mcp

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer binds the tools to the official MCP Go SDK. This file is the ONLY place
// (besides main) that imports the SDK, which is enforced by internal/archtest.
func NewServer(t *Tools, version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "orders-mcp-server", Version: version}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "findOrder",
		Description: "Returns status, total amount and last update of ONE order. " +
			"Use it when the user mentions a specific order id like ORD-1042.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in FindOrderInput) (*mcp.CallToolResult, OrderView, error) {
		out, err := t.FindOrder(ctx, scopes(req), in)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "listRecentOrders",
		Description: "Lists the most recent orders of ONE customer, newest first. " +
			"Use it when the user asks about the latest orders of a customer.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ListRecentOrdersInput) (*mcp.CallToolResult, OrderList, error) {
		out, err := t.ListRecentOrders(ctx, scopes(req), in)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "cancelOrder",
		Description: "Cancels ONE order that has not shipped yet. This action cannot be undone. " +
			"Only call it after the user explicitly confirmed the order id to cancel.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in CancelOrderInput) (*mcp.CallToolResult, OrderView, error) {
		out, err := t.CancelOrder(ctx, scopes(req), in)
		return nil, out, err
	})

	return server
}

// Handler exposes the server over Streamable HTTP.
func Handler(server *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
}

// scopes reads the scopes that auth.RequireBearerToken attached to the request.
func scopes(req *mcp.CallToolRequest) []string {
	if req == nil || req.Extra == nil || req.Extra.TokenInfo == nil {
		return nil
	}
	return req.Extra.TokenInfo.Scopes
}
