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
├── cmd/gobspect-mcp/main.go    # Entry point: server setup, StdioTransport, tool registration
├── internal/tools/
│   ├── input.go                # Resolve(data, file string) (io.Reader, error)
│   ├── schema.go               # gob_schema tool handler
│   ├── types.go                # gob_types tool handler
│   ├── decode.go               # gob_decode tool handler
│   ├── tabular.go              # gob_tabular tool handler
│   └── keys.go                 # gob_keys tool handler
└── PRD.md
```

The server registers five tools with `mcp.AddTool` and runs forever via `server.Connect` + `session.Wait()` over `StdioTransport`.

## Key Dependencies

- `github.com/codepuke/gobspect` — decode-only gob inspection library (local replace, see go.mod)
- `github.com/modelcontextprotocol/go-sdk` v0.8.0 — MCP server/client SDK

## Tool Input Convention

Every tool accepts exactly one of:
- `data` (string): Base64 Standard Encoding of raw gob bytes
- `file` (string): Absolute path to a `.gob` file on the filesystem

`internal/tools/input.Resolve` enforces this exclusivity and returns an `io.Reader`. All tool handlers call `input.Resolve` first.

## Code Style

- Standard Go conventions. Run `gofmt`, `go vet`, `staticcheck`.
- Error messages: lowercase, no trailing punctuation, include context: `"resolving input: %w"`.
- No panics in tool handlers. Return errors; the SDK converts them to tool errors.
- Use the generic `mcp.AddTool[In, Out]` function with typed input structs — do NOT use `server.AddTool` (raw handler).
- Input structs live in the same file as the handler. Keep tool input structs small and focused.
- Go 1.26+: use `any` not `interface{}`, use `slices`/`maps` stdlib packages.

## Testing

- Use `github.com/stretchr/testify` for assertions.
- Table-driven tests with `t.Run()`.
- Gob test fixtures live in `internal/tools/testdata/`.
- Use `go generate` via `testdata/generate.go` to regenerate fixtures when types change.
- Test each tool handler directly (not via MCP protocol); wire up an in-memory MCP connection only for integration tests.

## Things to Watch Out For

- `gob_decode` and `gob_tabular` share sorting logic with `gq`; do NOT import `cmd/gq` — reimplement the sort helpers cleanly in `internal/tools`.
- The tabular printer in `gq` is a good reference but must be reimplemented (package `main` is not importable).
- For `gob_tabular`, the `hetero` mode (first/reject/union/partition) follows the exact semantics documented in the gq README and `cmd/gq/tabular.go`.
- `query.Parse` panics on syntactically invalid expressions in the convenience functions (`Get`, `All`). Always use `query.Parse` + `query.AllPath`/`query.GetPath` in tool handlers so errors surface as tool errors, not panics.
- Output size: tools collect results in memory. LLM callers should use `limit` to bound output.
