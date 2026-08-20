# Project: gobspect-mcp

An MCP server wrapping the [`gobspect`](https://github.com/codepuke/gobspect) library. Exposes gob stream inspection, querying, and extraction capabilities as MCP tools for use with LLMs and MCP-compatible clients.

## PRD.md Execution

ALWAYS check off each box as tasks are completed.

When asked to implement a PRD.md:
1. Read the full PRD file before starting
2. Implement features in the order they appear — dependencies flow top to bottom
3. Do not start the next phase until the current one has passing tests
4. After all phases complete, run `go test ./...` and report results

## Architecture

```
gobspect-mcp/
├── cmd/gobspect-mcp/
│   ├── main.go                 # Entry point: server setup, StdioTransport, tool registration, version
│   └── version_test.go         # Pins the version const to go.mod and to the git tag
├── internal/tools/
│   ├── input.go                # Resolve(data, file, readLimit) (io.ReadCloser, error)
│   ├── limits.go               # read/output limits and the output-cap helpers
│   ├── schema.go               # gob_schema tool handler
│   ├── types.go                # gob_types tool handler
│   ├── decode.go               # gob_decode tool handler
│   ├── tabular.go              # gob_tabular tool handler
│   └── keys.go                 # gob_keys tool handler
├── docs/                       # Published to codepuke.com by the sync in the codepuke repo
└── PRD.md
```

The server registers five tools with `mcp.AddTool` and runs forever via `server.Connect` + `session.Wait()` over `StdioTransport`.

## Key Dependencies

- `github.com/codepuke/gobspect` — decode-only gob inspection library (tagged release, see go.mod)
- `github.com/modelcontextprotocol/go-sdk` — MCP server/client SDK

## Tool Input Convention

Every tool accepts exactly one of:
- `data` (string): Base64 Standard Encoding of raw gob bytes
- `file` (string): Absolute path to a `.gob` file on the filesystem

`tools.Resolve(data, file string, readLimit int64) (io.ReadCloser, error)` enforces this exclusivity. All tool handlers call it first and `defer r.Close()`.

Every tool also accepts `read_limit` and `output_limit`; see "Resource limits" below.

### Automatic decompression

`Resolve` hands the source to `gobspect/decompress.Reader`, which sniffs the leading magic bytes and removes one compression layer — gzip, zstd, xz, bzip2, or single-entry zip — so handlers always see raw gob bytes. Unrecognized input passes through byte-identical.

Detection is by **content, never by extension**, and it applies to `data` and `file` alike. Do not add extension dispatch back: a mislabeled name must not select a codec. Zip input is buffered fully in memory, which is why `Resolve` caps the source reader.

### Resource limits

All input is untrusted and the whole response is returned as one MCP text block, so both are bounded (see `internal/tools/limits.go`):

- `read_limit` — decompressed bytes, wired to `gobspect.WithReadLimit`, and also used as the source cap inside `Resolve`. Default 64 MiB, max 1 GiB.
- `output_limit` — response bytes. Default 1 MiB, max 16 MiB.

Zero is rejected for both; unlimited is the configuration these prevent. `gob_decode` stops between whole rendered values via the `errOutputLimit` sentinel returned from the pipeline sink; `gob_tabular` caps the printer's writer and trims to a line boundary; `gob_schema` truncates at a line boundary; `gob_types` and `gob_keys` emit a single JSON document, so they error rather than produce invalid JSON.

## Code Style

- Standard Go conventions. Run `gofmt`, `go vet`, `staticcheck`.
- Error messages: lowercase, no trailing punctuation, include context: `"decoding stream: %w"`. Note that `Resolve`'s errors are returned verbatim by handlers, not re-wrapped — tests assert on the unwrapped text.
- No panics in tool handlers. Return errors; the SDK converts them to tool errors.
- Use the generic `mcp.AddTool[In, Out]` function with typed input structs — do NOT use `server.AddTool` (raw handler).
- Input structs live in the same file as the handler. Keep tool input structs small and focused.
- Go 1.26+: use `any` not `interface{}`, use `slices`/`maps` stdlib packages.

## Testing

- Use `github.com/stretchr/testify` for assertions.
- Table-driven tests with `t.Run()`.
- Gob test fixtures live in `internal/tools/testdata/`.
- Regenerate fixtures with `cd internal/tools && go run testdata/generate.go` when types change. Do not commit a regenerated `map_value.gob` — gob's map byte order is unstable, so it diffs spuriously.
- Test each tool handler directly (not via MCP protocol); wire up an in-memory MCP connection only for integration tests.
- Fuzz targets live in `internal/tools/fuzz_test.go` and follow gobspect's convention: a first-line `// Fuzzing baseline: <date>. Ran <t>, no failures, <n> corpus entries.` comment, updated after each real run. Fuzzed bytes always go through the base64 `data` input — never fuzz caller-supplied file paths against the real filesystem.
- Where a tool claims equivalence to a gq invocation, prefer a parity assertion. `TestHandleTabular_PartitionMatchesGQ` cross-checks the real binary when `gq` is on `PATH` and skips otherwise.

## Releasing

The version reported to MCP clients must equal the repository tag. It is the one piece of release metadata not derived from anything else, so it drifts silently unless a release follows this checklist.

`serverVersion()` in `cmd/gobspect-mcp/main.go` prefers the module version the toolchain stamps into a binary installed with `go install ...@vX.Y.Z`, which comes from the tag and cannot drift. The `version` const is the fallback for a plain `go build`, and it is the value a human has to keep correct.

**This repo's version tracks the `github.com/codepuke/gobspect` version in go.mod, not its own sequential lineage.** The server is a thin wrapper, so the number is most useful to a caller as a statement of which library they are talking to. v0.1.2 was followed directly by v0.3.1, skipping v0.2.x entirely, to match gobspect v0.3.1.

To cut a release:

1. `grep gobspect go.mod` — that version, without the `v`, is the release version.
2. Set the `version` const in `cmd/gobspect-mcp/main.go` to it.
3. `go test ./...`. `TestVersionMatchesGoMod` fails if step 2 was missed.
4. Commit and push to `main`.
5. `git tag -a vX.Y.Z -m "vX.Y.Z" && git push origin vX.Y.Z`.
6. `go test ./cmd/...` again. `TestVersionMatchesGitTag` runs only on a tagged commit — this is the run that exercises it, and the last chance to catch a mismatch before the release notes go out.
7. `gh release create vX.Y.Z`.

When a gobspect upgrade lands, bumping the const is part of that change, not of a later release commit. `TestVersionMatchesGoMod` fails the moment go.mod moves without it.

## Things to Watch Out For

- The query→index→sort→offset→limit pipeline and gq's value renderer come from `gobspect/gq` (`gq.Pipeline`, `gq.Render`); decompression comes from `gobspect/decompress`; sorting and tabular output from `gobspect/sortval` and `gobspect/tabular`. Do NOT reimplement any of them — that duplication is exactly what v0.3.1 removed. `cmd/gq` remains unimportable (`package main`), but everything it does is now library code.
- `gq.Pipeline`'s zero `Index` selects only the **first** top-level value. Map a nil `*int` input to `gq.IndexAll`, never to 0.
- `gq.Pipeline` returns decode errors unwrapped and sink errors wrapped in `*gq.SinkError`. Handlers must re-add their own `"decoding stream: %w"` context and unwrap sink errors, or error text drifts.
- For `gob_tabular`, the `hetero` mode (first/reject/union/partition) follows the exact semantics documented in the gq README and the `gobspect/tabular` package. `gq.RunTabular` reads the printer's mode and sorts per struct-type partition in `partition` mode — do not sort globally before it.
- `query.Parse` panics on syntactically invalid expressions in the convenience functions (`Get`, `All`). Always use `query.Parse` + `query.AllPath`/`query.GetPath` in tool handlers so errors surface as tool errors, not panics.
- The `version` const in `cmd/gobspect-mcp/main.go` is not derived from the tag. Bump it in the same commit that bumps gobspect in go.mod, and see "Releasing" above before tagging.
- Output size: tools collect results in memory. `output_limit` bounds the response, but `limit` is still the better tool for LLM callers — it bounds the work, not just the text.
