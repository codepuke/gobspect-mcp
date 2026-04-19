package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/codepuke/gobspect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TypesInput is the input for the gob_types tool.
type TypesInput struct {
	Data       string `json:"data,omitempty"        jsonschema:"Base64-encoded gob bytes"`
	File       string `json:"file,omitempty"        jsonschema:"Absolute path to a gob file"`
	TimeFormat string `json:"time_format,omitempty" jsonschema:"Go time layout for time.Time values (default: RFC3339Nano)"`
}

func registerTypes(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "gob_types",
		Description: "Return type definitions from a gob stream as a JSON array. Provide data (base64) or file (path). Equivalent to 'gq -types'.",
	}, handleTypes)
}

func handleTypes(_ context.Context, _ *mcp.CallToolRequest, in TypesInput) (*mcp.CallToolResult, any, error) {
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

	stream := ins.Stream(r)
	if _, err := stream.Collect(); err != nil {
		return nil, nil, fmt.Errorf("decoding stream: %w", err)
	}

	out, err := json.MarshalIndent(stream.Types(), "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling types: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(out)}},
	}, nil, nil
}
