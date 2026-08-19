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
	Data        string `json:"data,omitempty"        jsonschema:"Base64-encoded gob bytes"`
	File        string `json:"file,omitempty"        jsonschema:"Absolute path to a gob file"`
	TimeFormat  string `json:"time_format,omitempty" jsonschema:"Go time layout for time.Time values (default: RFC3339Nano)"`
	ReadLimit   *int64 `json:"read_limit,omitempty"   jsonschema:"Max decompressed bytes to read from the input (default 67108864, max 1073741824)"`
	OutputLimit *int   `json:"output_limit,omitempty" jsonschema:"Max response bytes before truncation (default 1048576, max 16777216)"`
}

func registerTypes(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "gob_types",
		Description: "Return type definitions from a gob stream as a JSON array. Provide data (base64) or file (path). Equivalent to 'gq -types'.",
	}, handleTypes)
}

func handleTypes(_ context.Context, _ *mcp.CallToolRequest, in TypesInput) (*mcp.CallToolResult, any, error) {
	readLimit, outputLimit, err := resolveLimits(in.ReadLimit, in.OutputLimit)
	if err != nil {
		return nil, nil, err
	}

	r, err := Resolve(in.Data, in.File, readLimit)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()

	opts := []gobspect.Option{gobspect.WithReadLimit(readLimit)}
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

	// A truncated JSON document would not parse, so this response is refused
	// rather than cut.
	if len(out) > outputLimit {
		return nil, nil, fmt.Errorf("output of %d bytes exceeds output_limit of %d; raise output_limit", len(out), outputLimit)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(out)}},
	}, nil, nil
}
