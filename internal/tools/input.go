// Package tools implements the five gobspect MCP tool handlers.
package tools

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Resolve decodes base64 data or opens the named file and returns a ReadCloser
// over the raw gob bytes. Exactly one of data or file must be non-empty.
// Caller must close the returned reader.
func Resolve(data, file string) (io.ReadCloser, error) {
	if data != "" && file != "" {
		return nil, fmt.Errorf("provide either data or file, not both")
	}
	if data == "" && file == "" {
		return nil, fmt.Errorf("provide either data or file")
	}
	if data != "" {
		b, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			// Try without padding.
			b, err = base64.RawStdEncoding.DecodeString(data)
			if err != nil {
				return nil, fmt.Errorf("decoding base64 data: %w", err)
			}
		}
		return io.NopCloser(bytes.NewReader(b)), nil
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	return f, nil
}

// Register adds all gobspect-mcp tools to s.
func Register(s *mcp.Server) {
	registerSchema(s)
	registerTypes(s)
	registerDecode(s)
	registerKeys(s)
	registerTabular(s)
}
