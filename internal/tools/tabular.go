package tools

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/codepuke/gobspect"
	"github.com/codepuke/gobspect/query"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HeterogeneousMode controls what the tabular printer does when a row with a
// different struct type arrives after the first row has locked the schema.
type HeterogeneousMode int

const (
	HeterogeneousFirstWins HeterogeneousMode = iota
	HeterogeneousReject
	HeterogeneousUnion
	HeterogeneousPartition
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

	heteroMode, ok := parseHeteroMode(in.Hetero)
	if !ok {
		return nil, nil, fmt.Errorf("unknown hetero value %q; use first, reject, union, or partition", in.Hetero)
	}

	bytesFormat, ok := parseBytesFormat(in.Bytes)
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

	queryExpr := normalizeQuery(in.Query)
	path, err := query.Parse(queryExpr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid query expression %q: %w", in.Query, err)
	}

	var sortSpec SortSpec
	if in.Sort != "" {
		sortSpec, err = ParseSortSpec(in.Sort, in.SortDesc, in.SortFold, in.SortDropMissing)
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
	tp := newTabularPrinter(&buf,
		withDelimiter(delim),
		withNoHeaders(in.NoHeaders),
		withStream(stream),
		withBytesFormat(bytesFormat),
		withMaxBytes(maxBytes),
		withHeterogeneousMode(heteroMode),
	)

	idx := 0
	resultN := 0

	writeResult := func(v gobspect.Value) error {
		return tp.WriteValue(v)
	}

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
				allResults = append(allResults, result)
			}
			idx++
			if in.Index != nil && idx > *in.Index {
				break
			}
		}

		sorted := sortMatches(seqOf(allResults), sortSpec)
		for pos, result := range sorted {
			if pos < in.Offset {
				continue
			}
			if err := writeResult(result); err != nil {
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
				if err := writeResult(result); err != nil {
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

// parseHeteroMode converts a flag string to a HeterogeneousMode constant.
func parseHeteroMode(s string) (HeterogeneousMode, bool) {
	switch strings.ToLower(s) {
	case "", "first":
		return HeterogeneousFirstWins, true
	case "reject":
		return HeterogeneousReject, true
	case "union":
		return HeterogeneousUnion, true
	case "partition":
		return HeterogeneousPartition, true
	default:
		return HeterogeneousFirstWins, false
	}
}

// tabularOption is a functional option for newTabularPrinter.
type tabularOption func(*tabularPrinter)

func withDelimiter(r rune) tabularOption       { return func(tp *tabularPrinter) { tp.w.Comma = r } }
func withNoHeaders(b bool) tabularOption       { return func(tp *tabularPrinter) { tp.noHeaders = b } }
func withHeterogeneousMode(m HeterogeneousMode) tabularOption {
	return func(tp *tabularPrinter) { tp.heteroMode = m }
}
func withBytesFormat(f gobspect.BytesFormat) tabularOption {
	return func(tp *tabularPrinter) { tp.bytesFormat = f }
}
func withMaxBytes(n int) tabularOption  { return func(tp *tabularPrinter) { tp.maxBytes = n } }
func withStream(s *gobspect.Stream) tabularOption {
	return func(tp *tabularPrinter) { tp.stream = s }
}

type tabularPrinter struct {
	w          *csv.Writer
	rawWriter  io.Writer
	stream     *gobspect.Stream
	noHeaders  bool
	headerDone bool
	headers    []string

	heteroMode   HeterogeneousMode
	lockedTypeID int
	hasLock      bool
	lockedName   string

	projMode    bool
	rowCount    int
	bytesFormat gobspect.BytesFormat
	maxBytes    int
}

func newTabularPrinter(out io.Writer, opts ...tabularOption) *tabularPrinter {
	w := csv.NewWriter(out)
	w.Comma = ','
	tp := &tabularPrinter{
		w:           w,
		rawWriter:   out,
		bytesFormat: gobspect.BytesHex,
	}
	for _, o := range opts {
		o(tp)
	}
	return tp
}

func (tp *tabularPrinter) cellString(v gobspect.Value) string {
	if bv, ok := v.(gobspect.BytesValue); ok {
		return gobspect.FormatBytes(bv.V, tp.bytesFormat, tp.maxBytes)
	}
	return tabularCellString(v)
}

func (tp *tabularPrinter) WriteValue(v gobspect.Value) error {
	if iv, ok := v.(gobspect.InterfaceValue); ok {
		v = iv.Value
	}
	if sv, ok := v.(gobspect.StructValue); ok {
		return tp.writeStructRow(sv)
	}
	return tp.writeScalarRow(v)
}

func (tp *tabularPrinter) writeStructRow(sv gobspect.StructValue) error {
	tp.rowCount++

	if sv.TypeName == query.ProjectionTypeName {
		if !tp.headerDone {
			tp.headerDone = true
			tp.projMode = true
			tp.headers = fieldNames(sv.Fields)
			if !tp.noHeaders {
				if err := tp.w.Write(tp.headers); err != nil {
					return err
				}
			}
		}
		return tp.w.Write(tp.cellStrings(sv.Fields))
	}

	if !tp.headerDone {
		tp.headerDone = true
		tp.hasLock = true
		tp.lockedTypeID = sv.GobTypeID
		tp.lockedName = tp.typeName(sv.GobTypeID, sv.TypeName)
		tp.headers = tp.canonicalHeaders(sv)
		if !tp.noHeaders {
			if err := tp.w.Write(tp.headers); err != nil {
				return err
			}
		}
		return tp.w.Write(tp.sparseRow(sv))
	}

	if tp.hasLock && sv.GobTypeID != 0 && sv.GobTypeID != tp.lockedTypeID {
		incomingName := tp.typeName(sv.GobTypeID, sv.TypeName)
		switch tp.heteroMode {
		case HeterogeneousFirstWins:
			return nil

		case HeterogeneousReject:
			return fmt.Errorf("row %d has type %q but table is locked to %q; use a field projection like '.Items.*.Field1,Field2' to unify columns",
				tp.rowCount, incomingName, tp.lockedName)

		case HeterogeneousUnion:
			tp.growHeaders(sv)
			return tp.w.Write(tp.sparseRow(sv))

		case HeterogeneousPartition:
			tp.w.Flush()
			fmt.Fprintln(tp.rawWriter)
			tp.lockedTypeID = sv.GobTypeID
			tp.lockedName = incomingName
			tp.headers = tp.canonicalHeaders(sv)
			if !tp.noHeaders {
				if err := tp.w.Write(tp.headers); err != nil {
					return err
				}
			}
			return tp.w.Write(tp.sparseRow(sv))
		}
	}

	return tp.w.Write(tp.sparseRow(sv))
}

func (tp *tabularPrinter) growHeaders(sv gobspect.StructValue) {
	existing := make(map[string]bool, len(tp.headers))
	for _, h := range tp.headers {
		existing[h] = true
	}
	added := false
	for _, name := range tp.canonicalHeaders(sv) {
		if !existing[name] {
			tp.headers = append(tp.headers, name)
			existing[name] = true
			added = true
		}
	}
	if added && !tp.noHeaders {
		_ = tp.w.Write(tp.headers)
	}
}

func (tp *tabularPrinter) canonicalHeaders(sv gobspect.StructValue) []string {
	if tp.stream != nil && sv.GobTypeID != 0 {
		if ti, ok := tp.stream.TypeByID(sv.GobTypeID); ok && len(ti.Fields) > 0 {
			names := make([]string, len(ti.Fields))
			for i, f := range ti.Fields {
				names[i] = f.Name
			}
			return names
		}
	}
	return fieldNames(sv.Fields)
}

func (tp *tabularPrinter) typeName(id int, fallback string) string {
	if tp.stream != nil {
		if ti, ok := tp.stream.TypeByID(id); ok && ti.Name != "" {
			return ti.Name
		}
	}
	return fallback
}

func (tp *tabularPrinter) sparseRow(sv gobspect.StructValue) []string {
	byName := make(map[string]gobspect.Value, len(sv.Fields))
	for _, f := range sv.Fields {
		byName[f.Name] = f.Value
	}
	row := make([]string, len(tp.headers))
	for i, name := range tp.headers {
		if v, ok := byName[name]; ok {
			row[i] = tp.cellString(v)
		}
	}
	return row
}

func (tp *tabularPrinter) writeScalarRow(v gobspect.Value) error {
	tp.rowCount++
	if !tp.headerDone {
		tp.headerDone = true
		if !tp.noHeaders {
			if err := tp.w.Write([]string{"value"}); err != nil {
				return err
			}
		}
	}
	return tp.w.Write([]string{tp.cellString(v)})
}

func (tp *tabularPrinter) Flush() error {
	tp.w.Flush()
	return tp.w.Error()
}

func (tp *tabularPrinter) cellStrings(fields []gobspect.Field) []string {
	row := make([]string, len(fields))
	for i, f := range fields {
		row[i] = tp.cellString(f.Value)
	}
	return row
}

func fieldNames(fields []gobspect.Field) []string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	return names
}

// tabularCellString converts a single Value to a flat CSV cell string.
func tabularCellString(v gobspect.Value) string {
	switch v := v.(type) {
	case gobspect.StringValue:
		return v.V
	case gobspect.IntValue:
		return fmt.Sprintf("%d", v.V)
	case gobspect.UintValue:
		return fmt.Sprintf("%d", v.V)
	case gobspect.FloatValue:
		return fmt.Sprintf("%g", v.V)
	case gobspect.ComplexValue:
		if v.Imag >= 0 {
			return fmt.Sprintf("(%g+%gi)", v.Real, v.Imag)
		}
		return fmt.Sprintf("(%g%gi)", v.Real, v.Imag)
	case gobspect.BoolValue:
		if v.V {
			return "true"
		}
		return "false"
	case gobspect.NilValue:
		return ""
	case gobspect.BytesValue:
		return hex.EncodeToString(v.V)
	case gobspect.OpaqueValue:
		if v.Decoded != nil {
			if s, ok := v.Decoded.(string); ok {
				return s
			}
			return fmt.Sprint(v.Decoded)
		}
		return "(opaque)"
	case gobspect.InterfaceValue:
		return tabularCellString(v.Value)
	case gobspect.StructValue:
		return "(struct)"
	case gobspect.SliceValue:
		return "(slice)"
	case gobspect.ArrayValue:
		return "(array)"
	case gobspect.MapValue:
		return "(map)"
	default:
		return fmt.Sprintf("%v", v)
	}
}
