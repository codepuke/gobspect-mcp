package tools

import (
	"bytes"
	"context"
	"fmt"

	"github.com/codepuke/gobspect"
	"github.com/codepuke/gobspect/query"
	"github.com/codepuke/gobspect/sortval"
	"github.com/codepuke/gobspect/tabular"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TabularInput is the input for the gob_tabular tool.
type TabularInput struct {
	Data            string `json:"data,omitempty"              jsonschema:"Base64-encoded gob bytes"`
	File            string `json:"file,omitempty"              jsonschema:"Absolute path to a gob file"`
	Query           string `json:"query,omitempty"             jsonschema:"Path expression (e.g. .Items.*)"`
	Format          string `json:"format,omitempty"            jsonschema:"Output format: csv (default) or tsv"`
	Index           *int   `json:"index,omitempty"             jsonschema:"Use only the Nth top-level value (0-based); omit for all"`
	Offset          int    `json:"offset,omitempty"            jsonschema:"Skip the first N results"`
	Limit           int    `json:"limit,omitempty"             jsonschema:"Stop after N results (0 = no limit)"`
	Sort            string `json:"sort,omitempty"              jsonschema:"Comma-separated field names to sort by"`
	SortDesc        bool   `json:"sort_desc,omitempty"         jsonschema:"Reverse sort order"`
	SortFold        bool   `json:"sort_fold,omitempty"         jsonschema:"Case-insensitive string comparison in sort"`
	SortDropMissing bool   `json:"sort_drop_missing,omitempty" jsonschema:"Exclude rows missing all sort keys"`
	NoHeaders       bool   `json:"no_headers,omitempty"        jsonschema:"Suppress the header row"`
	Hetero          string `json:"hetero,omitempty"            jsonschema:"Heterogeneous-type handling: first (default), reject, union, or partition"`
	Bytes           string `json:"bytes,omitempty"             jsonschema:"Byte rendering: hex (default), base64, or literal"`
	MaxBytes        *int   `json:"max_bytes,omitempty"         jsonschema:"Truncation limit for byte slices (0 = no limit; default 64)"`
	TimeFormat      string `json:"time_format,omitempty"       jsonschema:"Go time layout for time.Time values (default: RFC3339Nano)"`
}

func registerTabular(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "gob_tabular",
		Description: "Decode a gob stream and return values as CSV or TSV rows. Provide data (base64) or file (path). Equivalent to 'gq -format csv/tsv [flags] [query]'.",
	}, handleTabular)
}

func handleTabular(_ context.Context, _ *mcp.CallToolRequest, in TabularInput) (*mcp.CallToolResult, any, error) {
	r, err := Resolve(in.Data, in.File)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()

	format := in.Format
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "tsv" {
		return nil, nil, fmt.Errorf("unknown format %q; use csv or tsv", format)
	}

	heteroMode, ok := tabular.ParseHeterogeneousMode(in.Hetero)
	if !ok {
		return nil, nil, fmt.Errorf("unknown hetero value %q; use first, reject, union, or partition", in.Hetero)
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

	var spec sortval.SortSpec
	if in.Sort != "" {
		spec, err = sortval.ParseSortSpec(in.Sort, in.SortDesc, in.SortFold, in.SortDropMissing)
		if err != nil {
			return nil, nil, err
		}
	}

	delim := rune(',')
	if format == "tsv" {
		delim = '\t'
	}

	stream := ins.Stream(r)
	var buf bytes.Buffer
	tp := tabular.NewPrinter(&buf,
		tabular.WithDelimiter(delim),
		tabular.WithNoHeaders(in.NoHeaders),
		tabular.WithStream(stream),
		tabular.WithBytesFormat(bytesFormat),
		tabular.WithMaxBytes(maxBytes),
		tabular.WithHeterogeneousMode(heteroMode),
	)

	idx := 0
	resultN := 0

	if len(spec.Keys) > 0 {
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
				allResults = append(allResults, result)
			}
			idx++
			if in.Index != nil && idx > *in.Index {
				break
			}
		}

		sorted := sortval.SortMatches(sortval.SeqOf(allResults), spec)
		for pos, result := range sorted {
			if pos < in.Offset {
				continue
			}
			if err := tp.WriteValue(result); err != nil {
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
				pos := resultN
				resultN++
				if pos < in.Offset {
					continue
				}
				if err := tp.WriteValue(result); err != nil {
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

	if err := tp.Flush(); err != nil {
		return nil, nil, fmt.Errorf("flushing tabular output: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: buf.String()}},
	}, nil, nil
}
