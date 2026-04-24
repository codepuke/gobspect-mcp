---
name: gobspect
description: "Inspect Go gob binary streams: schema, decode, query, CSV/TSV export."
trigger: /gobspect
---

# /gobspect

You have access to the **gobspect-mcp** server, which decodes and inspects Go `encoding/gob` binary streams without the original type definitions.

## When to invoke this skill

Use these tools when the user wants to:
- Understand the types inside a `.gob` file they don't have source for
- Extract specific fields or records from a gob stream
- Filter, sort, or paginate gob-encoded data
- Export gob records as CSV or TSV for further analysis

## The five tools at a glance

| Tool | Purpose | Analogous to |
|------|---------|-------------|
| `gob_schema` | Print Go-style type declarations embedded in the stream | `gq -schema` |
| `gob_types` | Return type metadata as a JSON array | `gq -types` |
| `gob_decode` | Decode values with optional query, sort, and pagination | `gq [flags] [query]` |
| `gob_tabular` | Export values as CSV or TSV | `gq -format csv/tsv` |
| `gob_keys` | List navigable keys at a given path | — |

---

## Input convention

Every tool accepts **exactly one** of:

- `data` (string): the raw gob bytes encoded as **standard base64** (RFC 4648, padding optional).
- `file` (string): absolute path to a `.gob` file on the filesystem the server can read.

Providing both, or neither, is an error.

**To base64-encode a file for inline use:**
```bash
# Linux / Mac
base64 < data.gob

# PowerShell (Windows)
[Convert]::ToBase64String([IO.File]::ReadAllBytes("C:\path\to\data.gob"))
```

If the user gives you a file path, always prefer `file` — it avoids transmitting large base64 blobs.

---

## Recommended workflow for an unknown gob stream

1. **`gob_schema`** — see the Go type declarations. This is the fastest way to understand what's inside.
2. **`gob_keys`** — if the schema is large, explore specific paths interactively.
3. **`gob_decode`** — extract values, optionally filtered by a query path.
4. **`gob_tabular`** — when the user wants structured rows for analysis or copy-paste.

---

## Query syntax

Path expressions are **dot-separated segments**. A leading `.` is optional (`.Field` and `Field` are equivalent).

### Navigation segments

| Segment | Meaning |
|---------|---------|
| `Field` | Named struct field, or string map key |
| `0`, `42` | Non-negative slice/array index |
| `-1`, `-2` | Negative index: `-1` = last element |
| `*` | All elements of a slice, array, or map |
| `..Field` | Recursive descent — `Field` at any depth, depth-first |
| `..[Filter]` | Wildcard descent — every node matching `Filter` at any depth |
| `A,B,C` | **Field projection** — returns an anonymous struct with only those fields |

### Filter syntax `[…]`

Filters narrow a slice, array, or map. They apply to the elements, not the container.

| Filter | Matches elements where… |
|--------|------------------------|
| `[Field!]` | `Field` is present and non-zero (gob omits zero-value fields) |
| `[Field!!]` | `Field` is absent |
| `[Field=pattern]` | `Field` is a string matching the glob (`*` = any, `?` = one char) |
| `[Field!=pattern]` | `Field` is a string NOT matching the glob |
| `[Field~pattern]` | `Field` is a slice/array/map containing a matching string |
| `[Field!~pattern]` | `Field` is a collection NOT containing a matching string |
| `[Field==value]` | `Field` is a number equal to `value` (also `<`, `>`, `<=`, `>=`) |
| `[Field==true]` | `Field` is a bool equal to `true` or `false` |
| `[F1=a]\|[F2=b]` | OR of two conditions |

**Tip:** Use `[Field=?*]` (not `[Field=*]`) to require a non-empty string — `?` demands at least one character.

### Field projections

`Field1,Field2,Field3` extracts named fields into an anonymous struct. Missing fields become `NilValue`.

Use `/` for nested fields within a projection: `Address/Zip` projects the `Zip` field of a nested `Address` struct (the output column name is `Zip`).

```
# Field projection in gob_decode
gob_decode(file: "…", query: "Orders.*.Customer,Total")

# Projection in gob_tabular — defines the CSV columns
gob_tabular(file: "…", query: "Items.*.SKU,Price,Address/Zip")
```

---

## Tool reference

### gob_schema

Returns Go-style type declarations as plain text. Always start here for an unknown file.

```json
{ "file": "/data/orders.gob" }
```

Output example:
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
```

Types marked `// GobEncoder` are opaque (e.g. `time.Time`, `math/big.Int`, `decimal.Decimal`).
The library decodes common standard-library types automatically.

### gob_keys

Returns a JSON array of navigable keys at a path. Use this when you want to discover field names
without reading the full schema, or to inspect a deeply nested node.

```json
{ "file": "/data/orders.gob", "query": "Orders.0" }
```

Returns: `["Customer", "ID", "Items", "PlacedAt"]`

`index` (default: 0) selects which top-level value to inspect.

### gob_decode

Decodes and formats values. The most versatile tool.

Key parameters:
| Parameter | Default | Notes |
|-----------|---------|-------|
| `query` | `""` | Path expression; empty = entire value |
| `format` | `"pretty"` | `"pretty"` or `"json"` |
| `index` | all values | `*int`: which top-level value to use |
| `limit` | 0 (no limit) | Stop after N results |
| `offset` | 0 | Skip first N results |
| `sort` | `""` | Comma-separated field names |
| `sort_desc` | false | Reverse sort order |
| `sort_fold` | false | Case-insensitive string sort |
| `sort_drop_missing` | false | Exclude rows missing all sort keys |
| `raw` | false | Omit quotes for top-level string results |
| `compact` | false | Compact JSON (no indentation) |
| `bytes` | `"hex"` | Byte rendering: `hex`, `base64`, `literal` |
| `max_bytes` | 64 | Truncation limit; 0 = no limit |
| `null_on_miss` | false | Return `"null"` instead of error on no match |
| `time_format` | RFC3339Nano | Go time layout for `time.Time` |

**JSON output** is newline-delimited (one JSON object per result line). Use `format: "json"` when the user wants to pipe results or inspect the AST.

### gob_tabular

Exports values as CSV or TSV rows. Column order follows the Go type definition.

Key parameters (in addition to the shared ones above):
| Parameter | Default | Notes |
|-----------|---------|-------|
| `format` | `"csv"` | `"csv"` or `"tsv"` |
| `no_headers` | false | Suppress header row |
| `hetero` | `"first"` | Mixed-type handling (see below) |

**Heterogeneous mode** — when the query matches structs of different Go types:
| Mode | Behavior |
|------|----------|
| `first` | Silently skip rows whose type differs from the first row |
| `reject` | Return an error on any type mismatch |
| `union` | Grow headers when new columns appear; earlier rows get empty cells |
| `partition` | Emit a blank line and a new header when the type changes |

### gob_types

Returns the full type metadata as a JSON array of `TypeInfo` objects. Use when you need
programmatic access to type IDs, field lists, and wire-format kinds.

```json
{ "file": "/data/orders.gob" }
```

---

## Examples

```
# What types are in this file?
gob_schema(file: "/data/orders.gob")

# Explore the first order's fields
gob_keys(file: "/data/orders.gob", query: "Orders.0")

# Print all customer names
gob_decode(file: "/data/orders.gob", query: "Orders.*.Customer")

# First 10 orders with Status "shipped", sorted by total descending
gob_decode(
  file: "/data/orders.gob",
  query: "Orders[Status=shipped]",
  sort: "Total",
  sort_desc: true,
  limit: 10
)

# Export line items as CSV, columns: SKU, Price, Quantity
gob_tabular(file: "/data/orders.gob", query: "Orders.*.Items.*.SKU,Price,Quantity")

# Find any Price field anywhere in the tree
gob_decode(file: "/data/orders.gob", query: "..Price")

# Inline base64 data
gob_schema(data: "Ag8BBf8C...")
```

---

## Common pitfalls

- **gob omits zero-value fields on the wire.** `[Field!]` tests for presence; a missing field was zero when encoded.
- **`query.Parse` panics on invalid paths.** The server catches this and returns a tool error — look for `"invalid query expression"` in error messages.
- **Map keys must be strings.** `map[int]T` entries are not navigable by path; use `*` to expand and filter instead.
- **Opaque types** (anything implementing `GobEncoder`) appear as their type name with raw hex bytes unless the library has a built-in decoder. Common built-ins: `time.Time`, `math/big.Int`, `math/big.Float`, `math/big.Rat`, `uuid.UUID`, `decimal.Decimal`.
- **Large results:** use `limit` to bound output for LLM consumption.
