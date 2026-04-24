# gobspect-mcp: Product Requirements Document

## 1. Overview

`gobspect-mcp` is an MCP server that wraps the [`gobspect`](https://github.com/codepuke/gobspect) library, exposing gob stream inspection as structured MCP tool calls. It enables LLMs and MCP-compatible clients to inspect, query, and extract data from Go `encoding/gob` binary streams without requiring the original type definitions.

The server runs as a subprocess communicating over stdin/stdout (`StdioTransport`), following the standard MCP stdio transport pattern.

## 2. Goals

- Expose all major `gq` CLI capabilities as typed MCP tools
- Accept gob data either as base64-encoded inline bytes or as a filesystem path
- Return structured, deterministic output suitable for LLM consumption
- Reuse `gobspect` and `gobspect/query` library logic; do not re-implement gob decoding

## 3. Non-Goals

- No encoding support — decode/inspect only
- No HTTP or WebSocket transport — stdio only
- No streaming of large result sets — results collected in memory
- No custom opaque decoder registration via MCP — built-in decoders only
- No compressed `data` handling (base64 bytes must be raw gob). File inputs are auto-decompressed by extension (see §5.3).

---

## 4. Architecture

### Binary Entry Point

`cmd/gobspect-mcp/main.go` — creates the MCP server, registers all tools, connects via `StdioTransport`, and blocks until the session ends.

```go
func main() {
    s := mcp.NewServer(&mcp.Implementation{Name: "gobspect-mcp", Version: "0.1.0"}, nil)
    tools.Register(s)
    session, err := s.Connect(context.Background(), &mcp.StdioTransport{}, nil)
    if err != nil { log.Fatal(err) }
    if err := session.Wait(); err != nil { log.Fatal(err) }
}
```

### Internal Packages

```
internal/tools/
  input.go      — Resolve(data, file string) (io.Reader, error); Register(s *mcp.Server)
  schema.go     — gob_schema handler
  types.go      — gob_types handler
  decode.go     — gob_decode handler
  tabular.go    — gob_tabular handler + tabularPrinter implementation
  keys.go       — gob_keys handler
  sort.go       — sorting helpers (SortSpec, ParseSortSpec, sortMatches) — shared by decode + tabular
  testdata/
    generate.go — go:generate helper that writes fixture .gob files
    *.gob       — generated fixtures
```

`Register(s *mcp.Server)` in `input.go` (or a dedicated `register.go`) calls `mcp.AddTool` for each of the five tools and is the single call site from `main.go`.

---

## 5. Data Input

### 5.1 Convention

Every tool accepts exactly one of:
- `data` (string): gob bytes encoded with standard base64 (RFC 4648, no padding required but accepted)
- `file` (string): absolute or relative path to a `.gob` file readable by the server process

If both are non-empty, `Resolve` returns an error: `"provide either data or file, not both"`.
If both are empty, `Resolve` returns an error: `"provide either data or file"`.

### 5.2 `input.Resolve` Signature

```go
// Resolve decodes base64 data or opens the named file and returns a reader
// over the raw gob bytes. For file inputs, a recognized compression extension
// causes the reader to be wrapped with a matching decompressor. Caller is
// responsible for closing file-based readers.
func Resolve(data, file string) (io.ReadCloser, error)
```

Returns an `io.ReadCloser` — for base64 data it wraps `bytes.Reader` with a no-op closer; for plain files it returns the open `*os.File`; for compressed files it returns a composite closer that closes both the decompressor and the underlying file. Callers `defer r.Close()`.

### 5.3 Automatic Decompression

When `file` is provided, `Resolve` matches the path's extension case-insensitively and wraps the reader with the matching decompressor so handlers always see raw gob bytes:

| Extension | Wrapper |
|-----------|---------|
| `.gz`, `.gzip` | `compress/gzip.NewReader` |
| `.zst`, `.zstd` | `github.com/klauspost/compress/zstd.NewReader` |
| `.bz2` | `compress/bzip2.NewReader` |
| `.xz` | `github.com/ulikunitz/xz.NewReader` |
| `.zip` | `archive/zip.OpenReader` — archive must contain exactly one file; otherwise returns `"zip archive must contain exactly one file"` |

Compound suffixes (`.gob.gz`) are resolved by the outermost extension. Unrecognized extensions are treated as raw gob. `data` input is never decompressed — clients must pass raw gob bytes encoded as base64.

---

## 6. Tool Specifications

### 6.1 `gob_schema`

**Purpose:** Print the Go-style type declarations embedded in a gob stream. Equivalent to `gq -schema`.

**Input struct:**
```go
type SchemaInput struct {
    Data       string `json:"data,omitempty"       jsonschema:"Base64-encoded gob bytes"`
    File       string `json:"file,omitempty"       jsonschema:"Path to gob file"`
    TimeFormat string `json:"time_format,omitempty" jsonschema:"Go time layout for time.Time values (default: RFC3339Nano)"`
}
```

**Behavior:**
1. Call `input.Resolve(in.Data, in.File)` to get an `io.Reader`.
2. Build `gobspect.New(opts...)` with `WithTimeFormat` if `TimeFormat` is set.
3. Call `ins.Stream(r).Schema()`.
4. Return `schema.String()` as a text content block.

**Output example:**
```
type LineItem struct {
  Price     Decimal  // GobEncoder
  Quantity  int
  SKU       string
}

type Order struct {
  Customer  string
  ID        uint
  Items     []LineItem
  PlacedAt  Time  // GobEncoder
}

type Decimal // GobEncoder
type Time    // GobEncoder
```

**Error cases:**
- Data/file resolution error → tool error with message
- Stream decode error → tool error with message

---

### 6.2 `gob_types`

**Purpose:** Return type definitions as a JSON array. Equivalent to `gq -types`.

**Input struct:**
```go
type TypesInput struct {
    Data       string `json:"data,omitempty"`
    File       string `json:"file,omitempty"`
    TimeFormat string `json:"time_format,omitempty"`
}
```

**Behavior:**
1. Resolve input.
2. `stream.Collect()` to drain the stream (errors from Collect are surfaced).
3. `json.MarshalIndent(stream.Types(), "", "  ")`.
4. Return JSON string as text content.

**Output example:**
```json
[
  {
    "id": -65,
    "name": "Order",
    "kind": "struct",
    "fields": [
      {"name": "Customer", "type_id": 6},
      {"name": "ID",       "type_id": 7},
      {"name": "Items",    "type_id": -66},
      {"name": "PlacedAt", "type_id": -67}
    ]
  },
  ...
]
```

---

### 6.3 `gob_decode`

**Purpose:** Decode gob values and return them in "pretty" or "json" format, with optional query path, index selection, pagination, and sorting. Equivalent to `gq` with `-format pretty` or `-format json`.

**Input struct:**
```go
type DecodeInput struct {
    Data            string `json:"data,omitempty"`
    File            string `json:"file,omitempty"`
    Query           string `json:"query,omitempty"            jsonschema:"Path expression, e.g. 'Orders.*.Customer.Name'"`
    Format          string `json:"format,omitempty"           jsonschema:"Output format: pretty or json (default: pretty)"`
    Index           int    `json:"index,omitempty"            jsonschema:"Only the Nth top-level value (0-based); -1 = all (default: -1)"`
    Offset          int    `json:"offset,omitempty"           jsonschema:"Skip first N results (default: 0)"`
    Limit           int    `json:"limit,omitempty"            jsonschema:"Stop after N results; 0 = no limit (default: 0)"`
    BytesFormat     string `json:"bytes_format,omitempty"     jsonschema:"Byte rendering: hex, base64, or literal (default: hex)"`
    MaxBytes        int    `json:"max_bytes,omitempty"        jsonschema:"Truncation limit for byte slices; 0 = no limit (default: 64)"`
    RawString       bool   `json:"raw_string,omitempty"       jsonschema:"Omit quotes for top-level string results (pretty only)"`
    Compact         bool   `json:"compact,omitempty"          jsonschema:"Compact JSON output (json format only)"`
    TimeFormat      string `json:"time_format,omitempty"`
    Sort            string `json:"sort,omitempty"             jsonschema:"Comma-separated field names to sort by"`
    SortDesc        bool   `json:"sort_desc,omitempty"        jsonschema:"Reverse sort order"`
    SortFold        bool   `json:"sort_fold,omitempty"        jsonschema:"Case-insensitive string sort"`
    SortDropMissing bool   `json:"sort_drop_missing,omitempty" jsonschema:"Exclude rows missing all sort keys"`
    NullOnMiss      bool   `json:"null_on_miss,omitempty"     jsonschema:"Return 'null' instead of error when query path not found"`
}
```

**Defaults (applied when zero value):**
- `Format`: `"pretty"`
- `Index`: `-1` (all values) — **note:** Go zero value is 0, so the struct needs a custom default; see §6.3.1
- `BytesFormat`: `"hex"`
- `MaxBytes`: `64`

**§6.3.1 Default for Index**

Since Go's zero value for `int` is `0` (meaning "first value only"), the JSON schema must declare `"default": -1` and the handler must treat missing/zero as -1. Use:

```go
type DecodeInput struct {
    ...
    Index *int `json:"index,omitempty" jsonschema:"Only the Nth value; omit or -1 = all"`
    ...
}
```

Or declare a sentinel: if `Index == 0 && !indexExplicitlySet`, treat as -1. Using `*int` (pointer) is the cleanest approach — nil means "all".

**Behavior:**
1. Resolve input.
2. Parse query with `query.Parse(in.Query)` — return tool error on parse failure.
3. Parse `BytesFormat`, `Format`, `Sort` — validate enums, return tool error on bad values.
4. Build format options: `WithBytesFormat`, `WithMaxBytes`.
5. If `Sort` is non-empty: collect all query results, sort, apply offset/limit, emit.
6. Else: stream values lazily, apply index filtering, apply offset/limit per result, emit.
7. Accumulate output in a `strings.Builder` or `bytes.Buffer`.
8. Return as single text content block.
9. If query was non-empty and nothing matched and `!NullOnMiss`: return tool error `"path %q not found"`.
10. If `NullOnMiss` and nothing matched: return `"null"`.

**Output (pretty):** Each matched value formatted with `gobspect.FormatTo`, followed by a newline. Multiple matches separated by blank lines would be noisy — just newlines.

**Output (json):** Each matched value as a JSON object from `gobspect.ToJSON` or `gobspect.ToJSONIndent`, one per line. For multiple results, output is newline-delimited JSON (ndjson), NOT a JSON array.

---

### 6.4 `gob_tabular`

**Purpose:** Export gob values as CSV or TSV, with column selection via field projection, sorting, pagination, and heterogeneous-type handling. Equivalent to `gq -format csv` or `gq -format tsv`.

**Input struct:**
```go
type TabularInput struct {
    Data            string `json:"data,omitempty"`
    File            string `json:"file,omitempty"`
    Query           string `json:"query,omitempty"`
    Format          string `json:"format,omitempty"            jsonschema:"csv or tsv (default: csv)"`
    Index           *int   `json:"index,omitempty"             jsonschema:"Only the Nth top-level value; nil = all"`
    Offset          int    `json:"offset,omitempty"`
    Limit           int    `json:"limit,omitempty"`
    NoHeaders       bool   `json:"no_headers,omitempty"        jsonschema:"Suppress header row"`
    Hetero          string `json:"hetero,omitempty"            jsonschema:"Heterogeneous type handling: first, reject, union, partition (default: first)"`
    BytesFormat     string `json:"bytes_format,omitempty"`
    MaxBytes        int    `json:"max_bytes,omitempty"`
    Sort            string `json:"sort,omitempty"`
    SortDesc        bool   `json:"sort_desc,omitempty"`
    SortFold        bool   `json:"sort_fold,omitempty"`
    SortDropMissing bool   `json:"sort_drop_missing,omitempty"`
    TimeFormat      string `json:"time_format,omitempty"`
    NullOnMiss      bool   `json:"null_on_miss,omitempty"`
}
```

**Hetero modes** (match `gq` semantics exactly):
- `first`: silently skip rows whose struct type differs from the first row
- `reject`: return tool error on type mismatch
- `union`: grow headers when new columns appear; earlier rows get empty cells
- `partition`: emit blank line + new header when type changes

**Behavior:**
1. Resolve input.
2. Parse query, validate enums.
3. Collect query matches (applying index/stream iteration as in `gob_decode`).
4. Apply sort if requested.
5. Apply offset/limit.
6. Feed results through a `tabularPrinter` writing to a `bytes.Buffer`.
7. Return buffer contents as text content.

**Column order:** follows Go type definition order for the first matched struct, matching `gq` behavior. Use `stream.TypeByID` to retrieve field order when available.

**Field projection:** When the query uses projection syntax (e.g. `Items.*.SKU,Price`), the tabular printer respects the projected column set.

---

### 6.5 `gob_keys`

**Purpose:** Discover navigable keys at a given path within a gob value. Useful for incremental schema exploration when the full schema is too large or the caller wants to navigate interactively.

**Input struct:**
```go
type KeysInput struct {
    Data  string `json:"data,omitempty"`
    File  string `json:"file,omitempty"`
    Query string `json:"query,omitempty"  jsonschema:"Path to navigate to before listing keys; empty = root"`
    Index *int   `json:"index,omitempty"  jsonschema:"Which top-level value to inspect (0-based); nil = 0"`
}
```

**Behavior:**
1. Resolve input.
2. `stream.Collect()` — drain the stream.
3. Select the target value: if `Index` is nil, use index 0; otherwise use `*Index`.
4. Parse `Query` with `query.Parse`.
5. Call `query.KeysPath(root, path)` to get the key list.
6. If `(nil, false)` — the node is scalar/opaque/nil; return `[]string{}`.
7. Return `json.Marshal(keys)` as text content.

**Output example:**
```json
["Customer", "ID", "Items", "PlacedAt"]
```

---

## 7. Sorting (shared, `internal/tools/sort.go`)

Reimplement the `SortSpec` / `ParseSortSpec` / `sortMatches` logic from `cmd/gq` (do not import from `main`). The implementation must match `gq`'s behavior exactly:

- `SortSpec` holds key names, descending flag, fold flag, drop-missing flag.
- `sortMatches` materializes all results, sorts by `SortSpec`, returns `[]gobspect.Value`.
- Kind-order total ordering: `NilValue < BoolValue < numeric < StringValue < BytesValue < OpaqueValue < composite`.
- Cross-kind numeric comparison uses `float64`.
- String comparison respects `SortFold` (Unicode case folding via `strings.EqualFold` / `unicode.ToLower`).
- When `SortDropMissing` is false, rows missing all sort keys sort lowest.

---

## 8. Implementation Phases

### Phase 1: Foundation
- [ ] `cmd/gobspect-mcp/main.go` — server entry point (10 lines)
- [ ] `internal/tools/input.go` — `Resolve` function + `Register` stub
- [ ] `internal/tools/schema.go` — `gob_schema` tool + unit tests
- [ ] `internal/tools/types.go` — `gob_types` tool + unit tests

**Acceptance criteria:**
- `go build ./cmd/gobspect-mcp` compiles without error
- `gob_schema` and `gob_types` return correct output on a simple fixture gob
- All tests pass: `go test ./...`

### Phase 2: Core Decode
- [ ] `internal/tools/sort.go` — `SortSpec`, `ParseSortSpec`, `sortMatches`
- [ ] `internal/tools/decode.go` — `gob_decode` tool + unit tests
- [ ] `internal/tools/keys.go` — `gob_keys` tool + unit tests

**Acceptance criteria:**
- `gob_decode` with no query returns all values from fixture
- `gob_decode` with query returns correct subset
- `gob_decode` with sort returns correctly ordered output
- `gob_keys` returns correct field names at root and nested paths
- All tests pass

### Phase 3: Tabular Output
- [ ] `internal/tools/tabular.go` — `gob_tabular` tool + tabularPrinter + unit tests

**Acceptance criteria:**
- `gob_tabular` with `format=csv` returns valid CSV (header + rows)
- `gob_tabular` with `no_headers=true` omits header row
- `gob_tabular` `hetero=union` grows columns correctly
- `gob_tabular` `hetero=partition` emits blank line + new header on type change
- Sort and pagination work correctly
- All tests pass

### Phase 4: Integration Tests
- [ ] `internal/tools/testdata/generate.go` — writes fixture `.gob` files
- [ ] End-to-end test via in-memory MCP connection calling each tool

**Acceptance criteria:**
- `go generate ./internal/tools/testdata/` regenerates fixtures deterministically
- Each tool has at least one end-to-end test through the MCP protocol
- All tests pass: `go test ./...`

---

## 9. Testing Strategy

### Fixtures

`internal/tools/testdata/generate.go` encodes known Go values using `encoding/gob` and writes them to `.gob` files. Required fixtures:

| File | Contents |
|---|---|
| `simple_struct.gob` | Single `Order{Customer: "Alice", ID: 42, Items: []LineItem{...}}` |
| `multi_value.gob` | Three `Order` values encoded sequentially |
| `nested.gob` | Deeply nested struct (3 levels) |
| `map_value.gob` | `map[string]string` |
| `opaque.gob` | Struct with `time.Time` and `math/big.Int` fields |
| `hetero.gob` | Two different struct types encoded in sequence (for hetero tests) |

### Tool Tests

Each tool has a `_test.go` file with table-driven tests covering:
- Happy path: correct output for valid input
- Bad base64 `data`: returns tool error
- Bad file path: returns tool error
- Both `data` and `file` set: returns tool error
- Neither set: returns tool error
- Invalid query expression: returns tool error
- Empty stream: returns correct output (empty schema, empty array, etc.)

### Integration Tests

`internal/tools/integration_test.go` uses `mcp.NewInMemoryTransports` to wire a real server session and calls each tool via `session.CallTool`, asserting on the returned content.

---

## 10. Configuration / Runtime

The binary takes no flags. Future extension points (not in scope):
- `--log-file` for debug logging
- `--read-limit` to pass through gobspect's `WithReadLimit`

---

## 11. Versioning

The server version in `mcp.Implementation` should be `"0.1.0"` initially. It will be bumped on breaking changes to tool schemas.
