// Command gobspect-mcp is an MCP server that exposes gob stream inspection
// capabilities via the Model Context Protocol.
package main

import (
	"context"
	"log"

	"github.com/codepuke/gobspect-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	s := mcp.NewServer(&mcp.Implementation{Name: "gobspect-mcp", Version: "0.1.0"}, nil)
	tools.Register(s)
	session, err := s.Connect(context.Background(), &mcp.StdioTransport{}, nil)
	if err != nil {
		log.Fatal(err)
	}
	if err := session.Wait(); err != nil {
		log.Fatal(err)
	}
}
