package tools

import (
	"context"
	"fmt"

	"github.com/codepuke/gobspect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SchemaInput is the input for the gob_schema tool.
type SchemaInput struct {
	Data       string `json:"data,omitempty"        jsonschema:"Base64-encoded gob bytes"`
	File       string `json:"file,omitempty"        jsonschema:"Absolute path to a gob file"`
	TimeFormat string `json:"time_format,omitempty" jsonschema:"Go time layout for time.Time values (default: RFC3339Nano)"`
}

func registerSchema(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "gob_schema",
		Description: "Print the Go-style type declarations embedded in a gob stream. Provide data (base64) or file (path). Equivalent to 'gq -schema'.",
	}, handleSchema)
}

func handleSchema(_ context.Context, _ *mcp.CallToolRequest, in SchemaInput) (*mcp.CallToolResult, any, error) {
	r, err := Resolve(in.Data, in.File)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()

	var opts []gobspect.Option
	if in.TimeFormat != "" {
		opts = append(opts, gobspect.WithTimeFormat(in.TimeFormat))
	}
	ins := gobspect.New(opts...)

	schema, err := ins.Stream(r).Schema()
	if err != nil {
		return nil, nil, fmt.Errorf("reading schema: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: schema.String()}},
	}, nil, nil
}
