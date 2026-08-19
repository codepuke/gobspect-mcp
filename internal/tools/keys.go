package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/codepuke/gobspect"
	"github.com/codepuke/gobspect/query"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// KeysInput is the input for the gob_keys tool.
type KeysInput struct {
	Data        string `json:"data,omitempty"        jsonschema:"Base64-encoded gob bytes"`
	File        string `json:"file,omitempty"        jsonschema:"Absolute path to a gob file"`
	Query       string `json:"query,omitempty"       jsonschema:"Path expression to navigate to before listing keys"`
	Index       *int   `json:"index,omitempty"       jsonschema:"Use only the Nth top-level value (0-based); omit for first"`
	TimeFormat  string `json:"time_format,omitempty" jsonschema:"Go time layout for time.Time values (default: RFC3339Nano)"`
	ReadLimit   *int64 `json:"read_limit,omitempty"   jsonschema:"Max decompressed bytes to read from the input (default 67108864, max 1073741824)"`
	OutputLimit *int   `json:"output_limit,omitempty" jsonschema:"Max response bytes before truncation (default 1048576, max 16777216)"`
}

func registerKeys(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "gob_keys",
		Description: "Return the navigable keys at a path in a gob stream as a JSON array. For structs: field names. For slices/arrays: index strings. For maps: string keys. Provide data (base64) or file (path).",
	}, handleKeys)
}

func handleKeys(_ context.Context, _ *mcp.CallToolRequest, in KeysInput) (*mcp.CallToolResult, any, error) {
	readLimit, outputLimit, err := resolveLimits(in.ReadLimit, in.OutputLimit)
	if err != nil {
		return nil, nil, err
	}

	r, err := Resolve(in.Data, in.File, readLimit)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()

	inspOpts := []gobspect.Option{gobspect.WithReadLimit(readLimit)}
	if in.TimeFormat != "" {
		inspOpts = append(inspOpts, gobspect.WithTimeFormat(in.TimeFormat))
	}
	ins := gobspect.New(inspOpts...)

	queryExpr := query.NormalizeQuery(in.Query)
	path, err := query.Parse(queryExpr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid query expression %q: %w", in.Query, err)
	}

	// Find the target top-level value (default: first = index 0).
	targetIdx := 0
	if in.Index != nil {
		if *in.Index < 0 {
			return nil, nil, fmt.Errorf("index must be non-negative")
		}
		targetIdx = *in.Index
	}

	stream := ins.Stream(r)
	idx := 0
	var target gobspect.Value
	for v, err := range stream.Values() {
		if err != nil {
			return nil, nil, fmt.Errorf("decoding stream: %w", err)
		}
		if idx == targetIdx {
			target = v
			break
		}
		idx++
	}
	if target == nil {
		return nil, nil, fmt.Errorf("stream has no value at index %d", targetIdx)
	}

	keys, ok := query.KeysPath(target, path)
	if !ok {
		return nil, nil, fmt.Errorf("path %q not found or node has no navigable keys", in.Query)
	}

	out, err := json.Marshal(keys)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling keys: %w", err)
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
