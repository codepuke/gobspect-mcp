// Package tools implements the five gobspect MCP tool handlers.
package tools

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/codepuke/gobspect/decompress"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Resolve decodes base64 data or opens the named file and returns a ReadCloser
// over the raw gob bytes. Exactly one of data or file must be non-empty.
//
// Compression is detected by sniffing the input's leading magic bytes, never by
// file extension, so gzip, zstd, xz, bzip2, and single-entry zip inputs are
// decompressed transparently on both paths and anything else passes through
// unchanged. readLimit caps the bytes read from the source; it bounds the
// in-memory buffering that zip input requires.
//
// Caller must close the returned reader.
func Resolve(data, file string, readLimit int64) (io.ReadCloser, error) {
	if data != "" && file != "" {
		return nil, fmt.Errorf("provide either data or file, not both")
	}
	if data == "" && file == "" {
		return nil, fmt.Errorf("provide either data or file")
	}

	if data != "" {
		b, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			b, err = base64.RawStdEncoding.DecodeString(data)
			if err != nil {
				return nil, fmt.Errorf("decoding base64 data: %w", err)
			}
		}
		dr, err := decompress.Reader(&limitedSource{r: bytes.NewReader(b), limit: readLimit})
		if err != nil {
			return nil, err
		}
		return dr, nil
	}

	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	dr, err := decompress.Reader(&limitedSource{r: f, limit: readLimit})
	if err != nil {
		f.Close()
		return nil, err
	}
	// decompress.Reader never closes its source, so the file stays ours.
	return composite{r: dr, closers: []io.Closer{dr, f}}, nil
}

// composite reads from r and closes every closer in order, reporting the first
// error encountered.
type composite struct {
	r       io.Reader
	closers []io.Closer
}

func (c composite) Read(p []byte) (int, error) { return c.r.Read(p) }

func (c composite) Close() error {
	var first error
	for _, cl := range c.closers {
		if err := cl.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Register adds all gobspect-mcp tools to s.
func Register(s *mcp.Server) {
	registerSchema(s)
	registerTypes(s)
	registerDecode(s)
	registerKeys(s)
	registerTabular(s)
}
