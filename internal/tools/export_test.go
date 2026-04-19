package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func HandleSchemaForTest(ctx context.Context, req *mcp.CallToolRequest, in SchemaInput) (*mcp.CallToolResult, any, error) {
	return handleSchema(ctx, req, in)
}

func HandleTypesForTest(ctx context.Context, req *mcp.CallToolRequest, in TypesInput) (*mcp.CallToolResult, any, error) {
	return handleTypes(ctx, req, in)
}

func HandleDecodeForTest(ctx context.Context, req *mcp.CallToolRequest, in DecodeInput) (*mcp.CallToolResult, any, error) {
	return handleDecode(ctx, req, in)
}

func HandleKeysForTest(ctx context.Context, req *mcp.CallToolRequest, in KeysInput) (*mcp.CallToolResult, any, error) {
	return handleKeys(ctx, req, in)
}

func HandleTabularForTest(ctx context.Context, req *mcp.CallToolRequest, in TabularInput) (*mcp.CallToolResult, any, error) {
	return handleTabular(ctx, req, in)
}

var TabularCellStringForTest = tabularCellString
