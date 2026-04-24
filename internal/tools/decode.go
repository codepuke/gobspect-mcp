package tools

import (
	"bytes"
	"context"
	"fmt"

	"github.com/codepuke/gobspect"
	"github.com/codepuke/gobspect/query"
	"github.com/codepuke/gobspect/sortval"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DecodeInput is the input for the gob_decode tool.
type DecodeInput struct {
	Data            string `json:"data,omitempty"              jsonschema:"Base64-encoded gob bytes"`
	File            string `json:"file,omitempty"              jsonschema:"Absolute path to a gob file"`
	Query           string `json:"query,omitempty"             jsonschema:"Path expression (e.g. .Field.Sub or .Items.*)"`
	Format          string `json:"format,omitempty"            jsonschema:"Output format: pretty (default) or json"`
	Index           *int   `json:"index,omitempty"             jsonschema:"Return only the Nth top-level value (0-based); omit for all"`
	Offset          int    `json:"offset,omitempty"            jsonschema:"Skip the first N results"`
	Limit           int    `json:"limit,omitempty"             jsonschema:"Stop after N results (0 = no limit)"`
	Sort            string `json:"sort,omitempty"              jsonschema:"Comma-separated field names to sort by"`
	SortDesc        bool   `json:"sort_desc,omitempty"         jsonschema:"Reverse sort order"`
	SortFold        bool   `json:"sort_fold,omitempty"         jsonschema:"Case-insensitive string comparison in sort"`
	SortDropMissing bool   `json:"sort_drop_missing,omitempty" jsonschema:"Exclude rows missing all sort keys"`
	Raw             bool   `json:"raw,omitempty"               jsonschema:"For string results, omit surrounding quotes"`
	Compact         bool   `json:"compact,omitempty"           jsonschema:"Compact JSON output (no indentation)"`
	NullOnMiss      bool   `json:"null_on_miss,omitempty"      jsonschema:"Emit null instead of an error when the query path is not found"`
	TimeFormat      string `json:"time_format,omitempty"       jsonschema:"Go time layout for time.Time values (default: RFC3339Nano)"`
	Bytes           string `json:"bytes,omitempty"             jsonschema:"Byte rendering: hex (default), base64, or literal"`
	MaxBytes        *int   `json:"max_bytes,omitempty"         jsonschema:"Truncation limit for byte slices (0 = no limit; default 64)"`
}

func registerDecode(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "gob_decode",
		Description: "Decode and query a gob stream, returning formatted values. Provide data (base64) or file (path). Equivalent to 'gq [flags] [query]'.",
	}, handleDecode)
}

func handleDecode(_ context.Context, _ *mcp.CallToolRequest, in DecodeInput) (*mcp.CallToolResult, any, error) {
	r, err := Resolve(in.Data, in.File)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()

	format := in.Format
	if format == "" {
		format = "pretty"
	}
	if format != "pretty" && format != "json" {
		return nil, nil, fmt.Errorf("unknown format %q; use pretty or json", format)
	}

	bytesFormat, ok := gobspect.ParseBytesFormat(in.Bytes)
	if !ok {
		return nil, nil, fmt.Errorf("unknown bytes value %q; use hex, base64, or literal", in.Bytes)
	}

	maxBytes := 64
	if in.MaxBytes != nil {
		maxBytes = *in.MaxBytes
	}

	var inspOpts []gobspect.Option
	if in.TimeFormat != "" {
		inspOpts = append(inspOpts, gobspect.WithTimeFormat(in.TimeFormat))
	}
	ins := gobspect.New(inspOpts...)

	queryExpr := query.NormalizeQuery(in.Query)
	path, err := query.Parse(queryExpr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid query expression %q: %w", in.Query, err)
	}

	var sortSpec sortval.SortSpec
	if in.Sort != "" {
		sortSpec, err = sortval.ParseSortSpec(in.Sort, in.SortDesc, in.SortFold, in.SortDropMissing)
		if err != nil {
			return nil, nil, err
		}
	}

	fmtOpts := []gobspect.FormatOption{
		gobspect.WithBytesFormat(bytesFormat),
		gobspect.WithMaxBytes(maxBytes),
	}

	stream := ins.Stream(r)
	var buf bytes.Buffer
	idx := 0
	anyMatch := false
	resultN := 0

	if len(sortSpec.Keys) > 0 {
		var allResults []gobspect.Value
		for v, err := range stream.Values() {
			if err != nil {
				return nil, nil, fmt.Errorf("decoding stream: %w", err)
			}
			if in.Index != nil && idx != *in.Index {
				idx++
				continue
			}
			for result := range query.AllPathSeq(v, path) {
				anyMatch = true
				allResults = append(allResults, result)
			}
			idx++
			if in.Index != nil && idx > *in.Index {
				break
			}
		}

		sorted := sortval.SortMatches(sortval.SeqOf(allResults), sortSpec)
		for pos, result := range sorted {
			if pos < in.Offset {
				continue
			}
			if err := writeDecodeValue(&buf, result, format, in.Raw, in.Compact, fmtOpts); err != nil {
				return nil, nil, err
			}
			resultN++
			if in.Limit > 0 && resultN >= in.Limit {
				break
			}
		}
	} else {
	outer:
		for v, err := range stream.Values() {
			if err != nil {
				return nil, nil, fmt.Errorf("decoding stream: %w", err)
			}
			if in.Index != nil && idx != *in.Index {
				idx++
				continue
			}
			for result := range query.AllPathSeq(v, path) {
				anyMatch = true
				pos := resultN
				resultN++
				if pos < in.Offset {
					continue
				}
				if err := writeDecodeValue(&buf, result, format, in.Raw, in.Compact, fmtOpts); err != nil {
					return nil, nil, err
				}
				if in.Limit > 0 && resultN-in.Offset >= in.Limit {
					break outer
				}
			}
			idx++
			if in.Index != nil && idx > *in.Index {
				break
			}
		}
	}

	if queryExpr != "" && !anyMatch {
		if in.NullOnMiss {
			buf.WriteString("null\n")
		} else {
			return nil, nil, fmt.Errorf("path %q not found", in.Query)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: buf.String()}},
	}, nil, nil
}

func writeDecodeValue(buf *bytes.Buffer, v gobspect.Value, format string, raw, compact bool, fmtOpts []gobspect.FormatOption) error {
	if format == "json" {
		var out []byte
		var err error
		if compact {
			out, err = gobspect.ToJSON(v)
		} else {
			out, err = gobspect.ToJSONIndent(v, "", "  ")
		}
		if err != nil {
			return fmt.Errorf("encoding JSON: %w", err)
		}
		buf.Write(out)
		buf.WriteByte('\n')
		return nil
	}

	// pretty
	if raw {
		target := v
		if iv, ok := target.(gobspect.InterfaceValue); ok {
			target = iv.Value
		}
		if sv, ok := target.(gobspect.StringValue); ok {
			buf.WriteString(sv.V)
			buf.WriteByte('\n')
			return nil
		}
	}
	if err := gobspect.FormatTo(buf, v, fmtOpts...); err != nil {
		return err
	}
	buf.WriteByte('\n')
	return nil
}
