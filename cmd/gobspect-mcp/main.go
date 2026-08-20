// Command gobspect-mcp is an MCP server that exposes gob stream inspection
// capabilities via the Model Context Protocol.
package main

import (
	"context"
	"log"
	"runtime/debug"
	"strings"

	"github.com/codepuke/gobspect-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is the fallback server version, used when the binary carries no
// module version of its own. It must equal the repository tag it ships in;
// TestVersionMatchesGitTag enforces that. It is independent of the gobspect
// version in go.mod — either can need a patch the other does not.
const version = "0.3.1"

// serverVersion prefers the module version the toolchain stamps into a binary
// installed with `go install ...@vX.Y.Z`, which is read from the tag and so
// cannot drift. A plain `go build` reports "(devel)" instead, and a binary
// built outside a module reports nothing; both fall back to the const.
func serverVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return version
}

func main() {
	s := mcp.NewServer(&mcp.Implementation{Name: "gobspect-mcp", Version: serverVersion()}, nil)
	tools.Register(s)
	session, err := s.Connect(context.Background(), &mcp.StdioTransport{}, nil)
	if err != nil {
		log.Fatal(err)
	}
	if err := session.Wait(); err != nil {
		log.Fatal(err)
	}
}
