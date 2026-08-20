---
title: Setup
---

# Setup

gobspect-mcp is a single static binary that speaks the MCP stdio transport: it
reads JSON-RPC from stdin and writes to stdout. It needs no configuration
file, no environment variables, and no command-line arguments. Installing it
is therefore two steps — put the binary somewhere, then tell your client where
it is.

## Install

You need Go 1.26 or later. Download it from [go.dev](https://go.dev/dl/).

```sh
go install github.com/codepuke/gobspect-mcp/cmd/gobspect-mcp@latest
```

`go install` places the binary in your Go bin directory:

| Platform | Default location |
|---|---|
| Linux / macOS | `~/go/bin/gobspect-mcp` |
| Windows | `%USERPROFILE%\go\bin\gobspect-mcp.exe` |

Confirm it landed and is executable:

```sh
ls -l ~/go/bin/gobspect-mcp
```

Most MCP clients do not inherit your interactive shell's `PATH`, so the safest
choice is to give every config below the **absolute path** to the binary
rather than a bare `gobspect-mcp`. The examples use
`/home/yourname/go/bin/gobspect-mcp`; substitute your own path throughout.

Running the binary directly is not useful on its own — with no client on the
other end of the pipe it simply waits for JSON-RPC input. Launch it from a
client instead.

## Claude Code

The CLI is the shortest path, and it works on all three platforms:

```sh
claude mcp add gobspect-mcp -- ~/go/bin/gobspect-mcp
```

```powershell
claude mcp add gobspect-mcp -- "$env:USERPROFILE\go\bin\gobspect-mcp.exe"
```

To configure it by hand instead, add the server to `.claude/settings.json` in
a project root, or to `~/.claude/settings.json` (`%USERPROFILE%\.claude\settings.json`
on Windows) to make it available in every project:

```json
{
  "mcpServers": {
    "gobspect-mcp": {
      "command": "/home/yourname/go/bin/gobspect-mcp"
    }
  }
}
```

On Windows, backslashes in JSON must be escaped:

```json
{
  "mcpServers": {
    "gobspect-mcp": {
      "command": "C:\\Users\\yourname\\go\\bin\\gobspect-mcp.exe"
    }
  }
}
```

## Claude Desktop

Edit the config file for your platform:

| Platform | Config file |
|---|---|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
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

```json
{
  "mcpServers": {
    "gobspect-mcp": {
      "command": "C:\\Users\\yourname\\go\\bin\\gobspect-mcp.exe"
    }
  }
}
```

Restart Claude Desktop after saving. The five `gob_*` tools then appear in the
tool list.

## Cursor

Edit `~/.cursor/mcp.json` for a user-wide server, or `.cursor/mcp.json` in a
project root to scope it to that project:

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

Reload the window after saving: `Ctrl+Shift+P` → "Developer: Reload Window".

## Aider

In `~/.aider.conf.yml`:

```yaml
mcp-servers:
  gobspect-mcp:
    command: /home/yourname/go/bin/gobspect-mcp
```

```yaml
mcp-servers:
  gobspect-mcp:
    command: C:\Users\yourname\go\bin\gobspect-mcp.exe
```

Or per invocation:

```sh
aider --mcp-server gobspect-mcp:/home/yourname/go/bin/gobspect-mcp
```

## Continue (VS Code / JetBrains)

In `~/.continue/config.json`:

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

## Any other MCP client

Because the server takes no arguments and reads no configuration, any client
supporting the stdio transport can launch it with:

```
command: /path/to/gobspect-mcp
args:    []
env:     {}
```

## Optional: the Claude Code skill

The repository ships a `/gobspect` skill that teaches Claude the tool
workflow, the query syntax, and the common inspection patterns — which tool to
reach for first, how to build a path expression, when to switch from
`gob_decode` to `gob_tabular`. It is a plain Markdown file, independent of the
server itself; the server works without it.

Fetch `skill/SKILL.md` from
[the repository](https://github.com/codepuke/gobspect-mcp/blob/main/skill/SKILL.md)
and save it into your skills directory:

```sh
mkdir -p ~/.claude/skills/gobspect
curl -fsSL https://raw.githubusercontent.com/codepuke/gobspect-mcp/main/skill/SKILL.md \
  -o ~/.claude/skills/gobspect/SKILL.md
```

```powershell
New-Item -ItemType Directory -Force "$env:USERPROFILE\.claude\skills\gobspect"
Invoke-WebRequest `
  -Uri https://raw.githubusercontent.com/codepuke/gobspect-mcp/main/skill/SKILL.md `
  -OutFile "$env:USERPROFILE\.claude\skills\gobspect\SKILL.md"
```

Then register it in `~/.claude/CLAUDE.md`, creating that file if it does not
exist:

```markdown
# gobspect
- **gobspect** (`~/.claude/skills/gobspect/SKILL.md`) - inspect Go gob binary streams. Trigger: `/gobspect`
When the user types `/gobspect`, invoke the Skill tool with `skill: "gobspect"` before doing anything else.
```

Typing `/gobspect` then activates it.

## Verifying the install

Ask your assistant to run `gob_schema` against any `.gob` file with an
absolute path. A stream that decodes returns Go-style type declarations. If
you get "provide either data or file", the tool is reachable and the input was
simply missing — which is itself confirmation the server is running.

Next: [Tools and queries](/docs/gobspect-mcp/tools).
