---
title: Overview
---

# gobspect-mcp

An MCP server that exposes Go [`encoding/gob`](https://pkg.go.dev/encoding/gob)
stream inspection as structured tool calls. It wraps the
[gobspect](/docs/gobspect/api) library, so an assistant can decode and
query an arbitrary gob stream without holding the original Go type
definitions — the type information the encoder embedded in the stream is
enough.

Gob is a self-describing format. Every stream carries the definitions of the
types it transmits before the values themselves, which is what makes
inspection without the source types possible at all. gobspect reads those
definitions and reconstructs a value tree; this server puts that tree behind
five tools.

The server speaks the stdio transport, requires no configuration, no
environment variables, and no arguments. See [Setup](/docs/gobspect-mcp/setup)
to install it and wire it into a client.

## The five tools

| Tool | What it does |
|---|---|
| `gob_schema` | Print the Go-style type declarations embedded in the stream |
| `gob_types` | Return the same type metadata as a JSON array, for programmatic use |
| `gob_decode` | Decode and query values, rendered as pretty text or newline-delimited JSON |
| `gob_tabular` | Decode and query values, rendered as CSV or TSV rows |
| `gob_keys` | List the navigable keys at a path, as a JSON string array |

`gob_schema` is almost always the right first call on an unfamiliar stream: it
tells you the type names and field names that every later query path is built
from. From there, `gob_keys` walks the shape one level at a time, and
`gob_decode` or `gob_tabular` extracts the data.

Full parameter tables and the query language are in
[Tools and queries](/docs/gobspect-mcp/tools).

## Input convention

Every tool takes its input the same way. Provide **exactly one** of:

- `data` — the raw gob bytes encoded as standard base64 (RFC 4648). Padding is
  optional; both padded and unpadded input decode.
- `file` — an absolute path to a file the server process can read.

Providing both, or neither, is an error. There is no default input and no
implicit working directory.

To produce base64 for the `data` form:

```sh
base64 < data.gob
```

```powershell
[Convert]::ToBase64String([IO.File]::ReadAllBytes("C:\path\to\data.gob"))
```

The `file` form avoids base64's 33% size inflation and keeps large streams out
of the conversation, so prefer it whenever the server and the file are on the
same machine.

## Automatic decompression

Compressed input is detected and decompressed transparently, on both `data`
and `file`:

| Format | Notes |
|---|---|
| gzip | |
| zstandard | |
| bzip2 | |
| xz | |
| zip | The archive must contain exactly one entry; it is buffered fully in memory |

Detection reads the input's leading magic bytes. **It never looks at the file
name.** A gzipped stream works whether it is called `orders.gob.gz`,
`orders.gob`, or `notes.txt`, and — more importantly — a file misleadingly
named `.gz` cannot trick the server into running the wrong codec.

Exactly one compression layer is removed. A gzipped zip archive decompresses
to a zip archive, not to its contents. Input that matches none of these
formats passes through byte-identical.

## Resource limits

All input to this server is untrusted, and the entire formatted result comes
back as a single MCP text response. Both the bytes read and the bytes returned
therefore have a ceiling. Every tool accepts both limits:

| Parameter | Default | Maximum | Bounds |
|---|---|---|---|
| `read_limit` | `67108864` (64 MiB) | `1073741824` (1 GiB) | **Decompressed** bytes read from the input |
| `output_limit` | `1048576` (1 MiB) | `16777216` (16 MiB) | Response bytes |

`read_limit` applies after decompression, which is the number that matters: a
small compressed input can expand enormously, and capping the compressed size
would not stop it.

Zero is rejected for both. Unlimited is precisely the configuration these
limits exist to prevent.

### What happens at the output limit

The tools differ, because their output has different natural boundaries:

| Tool | Behavior at `output_limit` |
|---|---|
| `gob_decode` | Stops between whole rendered values and appends a truncation notice |
| `gob_tabular` | Trims to a whole line and appends a truncation notice |
| `gob_schema` | Trims to a whole line and appends a truncation notice |
| `gob_types` | Returns an error |
| `gob_keys` | Returns an error |

`gob_types` and `gob_keys` each emit a single JSON document. Cutting one short
would produce text that does not parse, so they refuse rather than truncate.

When a response is truncated, raising `output_limit` is the last resort.
Narrowing `query` or setting `limit` is better: those bound the work, not just
the text, and they leave you with a result you can actually read.

## Opaque types

Some Go types implement `GobEncoder` and travel the wire as an opaque byte
blob rather than as structured fields. `gob_schema` marks these with a
`// GobEncoder` comment. gobspect decodes the common ones automatically —
`time.Time`, `math/big.Int`, `math/big.Float`, `math/big.Rat`,
`uuid.UUID`, and `decimal.Decimal`. Anything else renders as bytes. See
[Opaque type decoding](/docs/gobspect/opaque-types) for how that works.

## Related

- [gobspect](/docs/gobspect/api) — the underlying decode-only library
- [gq](https://github.com/codepuke/gobspect/tree/main/cmd/gq) — a command-line tool with the same capabilities,
  useful when a shell is a better fit than an assistant
