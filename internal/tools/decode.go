package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/codepuke/gobspect"
	"github.com/codepuke/gobspect/gq"
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
	ReadLimit       *int64 `json:"read_limit,omitempty"        jsonschema:"Max decompressed bytes to read from the input (default 67108864, max 1073741824)"`
	OutputLimit     *int   `json:"output_limit,omitempty"      jsonschema:"Max response bytes before truncation (default 1048576, max 16777216)"`
}

func registerDecode(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "gob_decode",
		Description: "Decode and query a gob stream, returning formatted values. Provide data (base64) or file (path). Equivalent to 'gq [flags] [query]'.",
	}, handleDecode)
}

func handleDecode(_ context.Context, _ *mcp.CallToolRequest, in DecodeInput) (*mcp.CallToolResult, any, error) {
	readLimit, outputLimit, err := resolveLimits(in.ReadLimit, in.OutputLimit)
	if err != nil {
		return nil, nil, err
	}

	r, err := Resolve(in.Data, in.File, readLimit)
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
	renderFormat, _ := gq.ParseFormat(format)
	if in.Index != nil && *in.Index < 0 {
		return nil, nil, fmt.Errorf("index must be non-negative")
	}

	bytesFormat, ok := gobspect.ParseBytesFormat(in.Bytes)
	if !ok {
		return nil, nil, fmt.Errorf("unknown bytes value %q; use hex, base64, or literal", in.Bytes)
	}

	maxBytes := 64
	if in.MaxBytes != nil {
		maxBytes = *in.MaxBytes
	}

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

	p := gq.Pipeline{
		Path:   path,
		Index:  gq.IndexAll,
		Offset: in.Offset,
		Limit:  in.Limit,
		Sort:   sortSpec,
	}
	if in.Index != nil {
		p.Index = *in.Index
	}

	renderOpts := gq.RenderOptions{
		Format:        renderFormat,
		Raw:           in.Raw,
		Compact:       in.Compact,
		FormatOptions: fmtOpts,
	}

	// Each result is rendered into a scratch buffer first, so the output limit
	// cuts between whole values rather than mid-record.
	var scratch bytes.Buffer
	truncated := false
	matched, err := p.Run(stream, func(v gobspect.Value) error {
		scratch.Reset()
		if err := gq.Render(&scratch, v, renderOpts); err != nil {
			return err
		}
		if buf.Len()+scratch.Len() > outputLimit {
			truncated = true
			return errOutputLimit
		}
		buf.Write(scratch.Bytes())
		return nil
	})
	if err != nil && !errors.Is(err, errOutputLimit) {
		// The pipeline returns decode errors unwrapped and sink errors wrapped
		// in SinkError; both need the context the handler has always added.
		var sinkErr *gq.SinkError
		if errors.As(err, &sinkErr) {
			return nil, nil, sinkErr.Err
		}
		return nil, nil, fmt.Errorf("decoding stream: %w", err)
	}

	if truncated {
		buf.WriteString(truncationNotice(outputLimit))
	}

	if queryExpr != "" && !matched {
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
