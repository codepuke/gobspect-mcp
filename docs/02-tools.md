---
title: Tools and Queries
---

# Tools and queries

Every tool takes exactly one of `data` (standard base64) or `file` (an
absolute path), plus `read_limit` and `output_limit`. Those shared parameters
are described in [Overview](/docs/gobspect-mcp/overview) and are not repeated
in each table below.

## `gob_schema`

Prints the Go-style type declarations embedded in the stream. This is the
right first call on a file you know nothing about: the names it returns are
the vocabulary every query path is built from.

```json
{ "file": "/data/orders.gob" }
```

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

Fields are listed in wire order, which is the encoder's field-ID order rather
than the declaration order of the original Go source. Types marked
`// GobEncoder` are opaque; see
[Opaque type decoding](/docs/gobspect/opaque-types).

| Parameter | Default | Description |
|---|---|---|
| `time_format` | RFC3339Nano | Go time layout for `time.Time` values |

## `gob_types`

Returns the same type information as a JSON array, with field IDs and wire
kinds exposed. Use it when something downstream needs to consume the shape
programmatically; use `gob_schema` when a human or an assistant is reading it.

```json
{ "file": "/data/orders.gob" }
```

| Parameter | Default | Description |
|---|---|---|
| `time_format` | RFC3339Nano | Go time layout for `time.Time` values |

Because the response is one JSON document, it cannot be truncated to fit
`output_limit` — an oversized result is an error instead.

## `gob_decode`

Decodes the stream and renders the values a query selects. This is the general
purpose tool; the others are specializations of it.

| Parameter | Default | Description |
|---|---|---|
| `query` | `""` | Path expression; empty selects the entire value |
| `format` | `"pretty"` | `"pretty"` or `"json"` |
| `index` | all | Use only the Nth top-level value (0-based); omit for all |
| `offset` | `0` | Skip the first N results |
| `limit` | `0` | Stop after N results (0 = no limit) |
| `sort` | `""` | Comma-separated field names to sort by |
| `sort_desc` | `false` | Reverse the sort order |
| `sort_fold` | `false` | Case-insensitive string comparison when sorting |
| `sort_drop_missing` | `false` | Exclude rows missing all sort keys |
| `raw` | `false` | For string results, omit the surrounding quotes |
| `compact` | `false` | Compact JSON, without indentation |
| `bytes` | `"hex"` | Byte rendering: `hex`, `base64`, or `literal` |
| `max_bytes` | `64` | Truncation limit for byte slices (0 = no limit) |
| `null_on_miss` | `false` | Emit `null` instead of erroring when the query matches nothing |
| `time_format` | RFC3339Nano | Go time layout for `time.Time` values |

With `format: "json"` the output is **newline-delimited JSON** — one document
per line — not a JSON array. Each result is a separate value, so there is no
enclosing bracket to parse.

`null_on_miss` only applies when `query` is non-empty. An empty query selects
the whole value and can never miss, so the flag has no effect there.

`limit` is worth reaching for early. It bounds the decoding work rather than
just the size of the response, which `output_limit` alone does not.

## `gob_tabular`

Renders selected values as CSV or TSV rows. Most useful after a field
projection has flattened the values into uniform columns.

| Parameter | Default | Description |
|---|---|---|
| `query` | `""` | Path expression; empty selects the entire value |
| `format` | `"csv"` | `"csv"` or `"tsv"` |
| `no_headers` | `false` | Suppress the header row |
| `hetero` | `"first"` | Mixed-type handling; see below |
| `index` | all | Use only the Nth top-level value (0-based); omit for all |
| `offset` | `0` | Skip the first N results |
| `limit` | `0` | Stop after N results (0 = no limit) |
| `sort` | `""` | Comma-separated field names to sort by |
| `sort_desc` | `false` | Reverse the sort order |
| `sort_fold` | `false` | Case-insensitive string comparison when sorting |
| `sort_drop_missing` | `false` | Exclude rows missing all sort keys |
| `bytes` | `"hex"` | Byte rendering: `hex`, `base64`, or `literal` |
| `max_bytes` | `64` | Truncation limit for byte slices (0 = no limit) |
| `time_format` | RFC3339Nano | Go time layout for `time.Time` values |

Note that `gob_tabular` is not simply `gob_decode` with a different renderer.
It has no `raw`, `compact`, or `null_on_miss` — none of them have a meaning
for tabular output — and its `format` selects `csv`/`tsv` rather than
`pretty`/`json`.

### Heterogeneous types

A query can match structs of several different Go types, which have no single
set of columns. `hetero` decides what to do:

| Mode | Behavior |
|---|---|
| `first` | Keep the first type matched; skip every row of a different type |
| `reject` | Return an error on the first type mismatch |
| `union` | Grow the header as new columns appear; earlier rows get empty cells |
| `partition` | Start a new table — blank line, then a fresh header — whenever the type changes |

`first` is the safe default for the common case where a stray value of another
type would otherwise corrupt the columns. `reject` turns that silent skip into
an error, which is what you want when uniformity is an assumption you would
rather have checked. `union` and `partition` both keep everything, differing
in whether you want one wide table or several narrow ones.

In `partition` mode, `sort` orders rows **within** each partition rather than
across the whole result, so the row order matches the tables actually printed.

## `gob_keys`

Returns the navigable keys at a path, as a JSON string array. For a struct,
the field names; for a slice or array, index strings (`"0"`, `"1"`, …); for a
`map[string]T`, the map keys.

This is the tool for walking an unfamiliar shape one level at a time, and the
only way to discover map keys — a schema shows that a field is a
`map[string]T`, but not which keys a particular value holds.

| Parameter | Default | Description |
|---|---|---|
| `query` | `""` | Path to navigate to before listing keys |
| `index` | `0` | Use only the Nth top-level value (0-based) |
| `time_format` | RFC3339Nano | Go time layout for `time.Time` values |

**`index` defaults differently here.** `gob_decode` and `gob_tabular` process
every top-level value in the stream when `index` is omitted; `gob_keys`
inspects only the first. A single set of keys is the useful answer for
navigation, and a stream's values are usually the same type anyway — but it
does mean one argument blob reused across tools will not mean the same thing
in each.

Like `gob_types`, this returns a single JSON document, so an oversized result
is an error rather than a truncation.

## Query syntax

A query is a dot-separated sequence of path segments. A leading dot is
optional: `.Orders.0` and `Orders.0` are the same path. An empty query
resolves to the root value.

### Navigation

| Segment | Navigates to |
|---|---|
| `Field` | A named struct field, or the entry of a `map[string]T` with that key |
| `0`, `42` | A slice or array element by index |
| `-1`, `-2` | A slice or array element counted from the end; `-1` is the last |
| `*` | Every element of a slice, array, or map |
| `..Field` | Recursive descent: every node named `Field`, at any depth |
| `..[Filter]` | Recursive descent keeping every node matching `Filter`, at any depth |
| `A,B,C` | Field projection: an anonymous struct holding only those fields |

Out-of-range indices yield no match rather than an error. Interface values are
unwrapped transparently at every step, so a `[]any` of structs navigates the
same as a `[]Order`.

### Filters

A filter segment keeps only the elements satisfying its condition:

| Filter | Keeps elements where… |
|---|---|
| `[Field!]` | `Field` is present |
| `[Field!!]` | `Field` is absent |
| `[Field=pattern]` | `Field` is a string matching the glob `pattern` |
| `[Field!=pattern]` | `Field` is a string **not** matching the glob |
| `[Field~pattern]` | `Field` is a collection containing a string matching the glob |
| `[Field!~pattern]` | `Field` is a collection containing no matching string |
| `[Field==value]` | `Field` is a number equal to `value`; also `<`, `>`, `<=`, `>=` |
| `[Field==true]` | `Field` is the bool `true`; `false` works the same way |
| `[F1=a]\|[F2=b]` | Either condition holds |

Globs follow `path.Match` semantics: `*` matches any run of characters,
including none, and `?` matches exactly one. That makes `[Field=*]` match the
empty string too, which is rarely what you want — use `[Field=?*]` to require
a non-empty string.

Comparison operators (`<`, `>`, `<=`, `>=`) apply to numbers only. `==` also
accepts `true` and `false` for bool fields.

When a pattern contains characters the parser would read as operators, wrap it
in double quotes: `[Formula="a<b"]`.

The `~` filter tests collection membership. It checks only the string entries
of a slice, array, or map, silently skipping anything else, so
`[Tags~prod*]` keeps elements whose `Tags` contains at least one string
starting with `prod`.

### Field projection

A comma-separated list of names extracts a subset of a struct into a uniform
anonymous struct. This is what turns a nested value tree into something with
columns:

```
Orders.*.ID,Customer,Total
```

Within a projection, `/` reaches into a nested struct, and the column takes
the last component's name:

```
Orders.*.Customer,Address/Zip
```

That yields two columns, `Customer` and `Zip`.

The `/` form only means nested navigation **inside a projection** — that is,
in a segment containing a comma. A bare segment containing a slash and no
comma is read as a literal field or map key, because string map keys are
allowed to contain slashes. To reach a single nested value without projecting,
use an ordinary dot: `Orders.0.Address.Zip`.

### Examples

```
                              # empty query: every value in the stream
Orders                        # one struct field
Orders.*                      # every element of a slice
Orders.2                      # the third order, 0-based
Orders.-1                     # the last order
Orders.0.Customer             # one field of one order
Orders.*.Customer             # that field from every order
Orders[Status=shipped]        # only the orders that shipped
Orders[Total>100]             # only the orders over 100
Orders[Status=shipped][Total>100]   # both conditions
..Price                       # every Price field, at any depth
Orders.*.ID,Customer,Total    # three columns, ready for gob_tabular
Orders.*.Customer,Address/Zip # projection reaching into a nested struct
```

### A worked pass over an unknown file

```json
{ "file": "/data/orders.gob" }
```

Call `gob_schema` first to learn that the stream holds `Order` values with an
`Items []LineItem` field. Then `gob_keys` with `"query": "Orders.0"` confirms
the field names on a real value. Then narrow with `gob_decode`:

```json
{ "file": "/data/orders.gob", "query": "Orders[Total>100].Customer", "limit": 20 }
```

And once the shape is settled, export it:

```json
{ "file": "/data/orders.gob", "query": "Orders.*.ID,Customer,Total", "format": "csv" }
```
