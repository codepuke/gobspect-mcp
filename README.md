# gobspect-mcp

An MCP server that exposes Go [`encoding/gob`](https://pkg.go.dev/encoding/gob) stream inspection as structured tool calls. Wraps the [`gobspect`](https://github.com/codepuke/gobspect) library — decode and query arbitrary gob streams without the original type definitions.

Five tools are exposed:

| Tool | What it does |
|------|-------------|
| `gob_schema` | Print Go-style type declarations embedded in a stream |
| `gob_types` | Return type metadata as a JSON array |
| `gob_decode` | Decode and query values (pretty or JSON output) |
| `gob_tabular` | Export records as CSV or TSV |
| `gob_keys` | List navigable keys at a given path |

---

## Installation

### Prerequisites

Go 1.26 or later. Download from [go.dev](https://go.dev/dl/).

### Install the binary

```sh
go install github.com/codepuke/gobspect-mcp/cmd/gobspect-mcp@latest
```

The binary is placed in your Go bin directory:

| Platform | Default location |
|----------|-----------------|
| Linux / Mac | `~/go/bin/gobspect-mcp` |
| Windows | `%USERPROFILE%\go\bin\gobspect-mcp.exe` |

Make sure that directory is in your `PATH`, or use the full path when configuring MCP clients.

### Build from source

```sh
git clone https://github.com/codepuke/gobspect-mcp.git
cd gobspect-mcp
go build -o dist/gobspect-mcp ./cmd/gobspect-mcp   # Linux / Mac
go build -o dist\gobspect-mcp.exe .\cmd\gobspect-mcp  # Windows (PowerShell)
```

---

## MCP Client Setup

The server speaks the stdio transport — it reads JSON-RPC from stdin and writes to stdout. Any MCP-compatible client can use it.

In all examples below, replace the binary path with your actual install location.

---

### Claude Code

**Via the CLI** (recommended — works on Linux, Mac, Windows):

```sh
# Linux / Mac
claude mcp add gobspect-mcp -- ~/go/bin/gobspect-mcp

# Windows (PowerShell)
claude mcp add gobspect-mcp -- "$env:USERPROFILE\go\bin\gobspect-mcp.exe"
```

**Via project settings** (`.claude/settings.json` in your project root):

```json
{
  "mcpServers": {
    "gobspect-mcp": {
      "command": "/home/yourname/go/bin/gobspect-mcp"
    }
  }
}
```

Windows equivalent:

```json
{
  "mcpServers": {
    "gobspect-mcp": {
      "command": "C:\\Users\\yourname\\go\\bin\\gobspect-mcp.exe"
    }
  }
}
```

**Via user settings** (`~/.claude/settings.json` on Linux/Mac, `%USERPROFILE%\.claude\settings.json` on Windows) — applies to all projects.

#### Claude Code skill

A `/gobspect` skill is available that teaches Claude the tool workflow, query syntax, and common patterns. To install it, copy `skill/SKILL.md` to your Claude Code skills directory:

```sh
# Linux / Mac
mkdir -p ~/.claude/skills/gobspect
cp skill/SKILL.md ~/.claude/skills/gobspect/SKILL.md
```

```powershell
# Windows (PowerShell)
New-Item -ItemType Directory -Force "$env:USERPROFILE\.claude\skills\gobspect"
Copy-Item skill\SKILL.md "$env:USERPROFILE\.claude\skills\gobspect\SKILL.md"
```

Then add this to your `~/.claude/CLAUDE.md` (create it if it doesn't exist):

```markdown
# gobspect
- **gobspect** (`~/.claude/skills/gobspect/SKILL.md`) - inspect Go gob binary streams. Trigger: `/gobspect`
When the user types `/gobspect`, invoke the Skill tool with `skill: "gobspect"` before doing anything else.
```

Type `/gobspect` in Claude Code to activate it. Claude will know when to use each tool and how to build query paths.

---

### Claude Desktop

Edit the config file for your platform:

| Platform | Config file |
|----------|-------------|
| Mac | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |
| Linux | `~/.config/Claude/claude_desktop_config.json` |

```json
{
  "mcpServers": {
    "gobspect-mcp": {
      "command": "/home/yourname/go/bin/gobspect-mcp"
    }
  }
}
```

Windows:

```json
{
  "mcpServers": {
    "gobspect-mcp": {
      "command": "C:\\Users\\yourname\\go\\bin\\gobspect-mcp.exe"
    }
  }
}
```

Restart Claude Desktop after saving. The `gobspect-mcp` tools will appear in the tool list.

---

### Cursor

Edit `~/.cursor/mcp.json` for user-wide config, or `.cursor/mcp.json` in your project root:

```json
{
  "mcpServers": {
    "gobspect-mcp": {
      "command": "/home/yourname/go/bin/gobspect-mcp",
      "args": []
    }
  }
}
```

Reload the Cursor window after saving (`Ctrl+Shift+P` → "Developer: Reload Window").

---

### Aider

Aider supports MCP servers via its configuration file (`~/.aider.conf.yml`) or command-line flags.

**Config file** (`~/.aider.conf.yml`):

```yaml
mcp-servers:
  gobspect-mcp:
    command: /home/yourname/go/bin/gobspect-mcp
```

Windows:

```yaml
mcp-servers:
  gobspect-mcp:
    command: C:\Users\yourname\go\bin\gobspect-mcp.exe
```

**Command line**:

```sh
aider --mcp-server gobspect-mcp:/home/yourname/go/bin/gobspect-mcp
```

---

### Continue (VS Code / JetBrains)

Edit `~/.continue/config.json`:

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "transport": {
          "type": "stdio",
          "command": "/home/yourname/go/bin/gobspect-mcp",
          "args": []
        }
      }
    ]
  }
}
```

---

### Other MCP clients

The server requires no environment variables, no config files, and no arguments. Any client that supports stdio MCP transport can launch it with:

```
command: /path/to/gobspect-mcp
args:    []
env:     {}
```

---

## Tool Reference

### Input convention

Every tool accepts **exactly one** of:

- `data` (string): the raw gob bytes encoded as **standard base64** (RFC 4648, padding optional).
- `file` (string): absolute path to a `.gob` file the server process can read.

Providing both, or neither, returns an error.

Encoding a file for inline use:

```sh
# Linux / Mac
base64 < data.gob

# PowerShell
[Convert]::ToBase64String([IO.File]::ReadAllBytes("C:\path\to\data.gob"))
```

#### Automatic decompression

When using `file`, the server inspects the path's extension (case-insensitive) and transparently decompresses on read:

| Extension | Format |
|-----------|--------|
| `.gz`, `.gzip` | gzip |
| `.zst`, `.zstd` | zstandard |
| `.bz2` | bzip2 |
| `.xz` | xz |
| `.zip` | zip (archive must contain exactly one entry) |

So `/data/orders.gob.gz` and `/data/orders.gob` work identically. Compound extensions resolve on the outermost suffix only. The `data` parameter is always treated as raw gob bytes — decompress client-side before base64-encoding.

---

### `gob_schema`

Print Go-style type declarations. Always the right first call on an unknown file.

```json
{ "file": "/data/orders.gob" }
```

Output:
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

Types annotated `// GobEncoder` are opaque — the library auto-decodes common ones (`time.Time`, `math/big.Int`, `math/big.Float`, `math/big.Rat`, `uuid.UUID`, `decimal.Decimal`).

Optional parameters: `time_format` (Go time layout, default RFC3339Nano).

---

### `gob_types`

Return type metadata as a JSON array. For programmatic inspection of field IDs and wire kinds.

```json
{ "file": "/data/orders.gob" }
```

---

### `gob_decode`

Decode and query values. The most versatile tool.

| Parameter | Default | Description |
|-----------|---------|-------------|
| `data` / `file` | — | Input source (one required) |
| `query` | `""` | Path expression (empty = entire value) |
| `format` | `"pretty"` | `"pretty"` or `"json"` |
| `index` | all | Which top-level value to use (0-based) |
| `limit` | 0 | Stop after N results (0 = no limit) |
| `offset` | 0 | Skip first N results |
| `sort` | `""` | Comma-separated field names to sort by |
| `sort_desc` | false | Reverse sort order |
| `sort_fold` | false | Case-insensitive string sort |
| `sort_drop_missing` | false | Exclude rows missing all sort keys |
| `raw` | false | Omit quotes for top-level string values |
| `compact` | false | Compact JSON (no indentation) |
| `bytes` | `"hex"` | Byte rendering: `hex`, `base64`, `literal` |
| `max_bytes` | 64 | Truncation limit for byte slices (0 = no limit) |
| `null_on_miss` | false | Return `"null"` instead of error when path not found |
| `time_format` | RFC3339Nano | Go time layout for `time.Time` |

JSON output is **newline-delimited** (one object per line), not a JSON array.

---

### `gob_tabular`

Export values as CSV or TSV.

| Parameter | Default | Description |
|-----------|---------|-------------|
| `format` | `"csv"` | `"csv"` or `"tsv"` |
| `no_headers` | false | Suppress the header row |
| `hetero` | `"first"` | Mixed-type handling (see below) |
| + all `gob_decode` parameters | | |

**Heterogeneous mode** — controls what happens when the query matches structs of different Go types:

| Mode | Behavior |
|------|----------|
| `first` | Skip rows whose type differs from the first matched type |
| `reject` | Return an error on any type mismatch |
| `union` | Grow headers as new columns appear; earlier rows get empty cells |
| `partition` | Emit a blank line and a new header when the type changes |

---

### `gob_keys`

List navigable keys at a given path. Returns a JSON string array.

| Parameter | Default | Description |
|-----------|---------|-------------|
| `query` | `""` | Path to navigate to before listing keys |
| `index` | 0 | Which top-level value to inspect |
| + `data` / `file` | — | Input source |

For a struct: field names. For a slice/array: index strings (`"0"`, `"1"`, …). For a `map[string]T`: map keys.

---

## Query Syntax

Expressions are dot-separated path segments. A leading `.` is optional (`.Field` = `Field`).

### Basic navigation

| Segment | Navigates to |
|---------|-------------|
| `Field` | Named struct field or string map key |
| `0`, `-1` | Slice/array index (0 = first, -1 = last) |
| `*` | All elements of a slice, array, or map |
| `..Field` | Recursive descent: `Field` anywhere in the subtree |
| `..[Filter]` | Recursive descent keeping nodes matching `Filter` |
| `A,B,C` | Field projection: anonymous struct with only those fields |

### Filters

| Filter | Keeps elements where… |
|--------|----------------------|
| `[Field!]` | `Field` is present and non-zero |
| `[Field!!]` | `Field` is absent |
| `[Field=pattern]` | `Field` is a string matching the glob (`*` = any chars, `?` = one char) |
| `[Field!=pattern]` | `Field` is a string NOT matching the glob |
| `[Field~pattern]` | `Field` is a collection containing a matching string |
| `[Field!~pattern]` | `Field` is a collection NOT containing a matching string |
| `[Field==value]` | `Field` is a number equal to `value` (also `<`, `>`, `<=`, `>=`) |
| `[Field==true]` | `Field` is the bool `true` |
| `[F1=a]\|[F2=b]` | OR of two conditions |

Use double quotes inside filters when the pattern contains operator characters: `[Formula="a<b"]`.

Use `[Field=?*]` (not `[Field=*]`) to require a non-empty string — `?` requires at least one character.

### Field projection

Comma-separated field names extract a subset of a struct into a uniform anonymous struct. Useful for CSV export and focusing on specific columns:

```
Orders.*.Customer,Total,Status
Items.*.SKU,Price,Address/Zip
```

Use `/` within a projection to reach a nested field (`Address/Zip` → column named `Zip`).

### Examples

```
# All values in the stream
(empty query)

# Struct field
Orders

# All elements of a slice
Orders.*

# The third order (0-based)
Orders.2

# Last order
Orders.-1

# A single field from the first order
Orders.0.Customer

# All customer names
Orders.*.Customer

# Orders where Status is "shipped"
Orders[Status=shipped]

# Orders over $100
Orders[Total>100]

# Recursive: find every Price field at any depth
..Price

# Project specific columns for CSV
Orders.*.ID,Customer,Total

# Nested projection: pull Zip out of a nested Address
Orders.*.Customer,Address/Zip
```

---

## Related projects

- [`gobspect`](https://github.com/codepuke/gobspect) — the underlying decode library
- [`gq`](https://github.com/codepuke/gobspect/tree/main/cmd/gq) — command-line tool with the same capabilities

## License

[MIT](LICENSE.txt)
