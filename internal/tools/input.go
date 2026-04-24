// Package tools implements the five gobspect MCP tool handlers.
package tools

import (
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ulikunitz/xz"
)

// Resolve decodes base64 data or opens the named file and returns a ReadCloser
// over the raw gob bytes. Exactly one of data or file must be non-empty.
// When file has a recognized compression extension (.gz, .gzip, .zst, .zstd,
// .bz2, .xz, .zip), the reader is transparently wrapped with a matching
// decompressor. Caller must close the returned reader.
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
			b, err = base64.RawStdEncoding.DecodeString(data)
			if err != nil {
				return nil, fmt.Errorf("decoding base64 data: %w", err)
			}
		}
		return io.NopCloser(bytes.NewReader(b)), nil
	}
	return openMaybeCompressed(file)
}

// openMaybeCompressed opens file and, based on its extension, returns either
// the raw file reader or a decompressing wrapper that closes the underlying
// file when closed.
func openMaybeCompressed(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case "":
		return f, nil
	case ".gz", ".gzip":
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("opening gzip stream: %w", err)
		}
		return composite{r: gz, closers: []io.Closer{gz, f}}, nil
	case ".zst", ".zstd":
		zr, err := zstd.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("opening zstd stream: %w", err)
		}
		return composite{r: zr, closers: []io.Closer{zstdCloser{zr}, f}}, nil
	case ".bz2":
		return composite{r: bzip2.NewReader(f), closers: []io.Closer{f}}, nil
	case ".xz":
		xr, err := xz.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("opening xz stream: %w", err)
		}
		return composite{r: xr, closers: []io.Closer{f}}, nil
	case ".zip":
		return openZip(f, path)
	default:
		return f, nil
	}
}

// openZip reads f as a zip archive. The archive must contain exactly one
// file entry; its contents are returned as the reader.
func openZip(f *os.File, path string) (io.ReadCloser, error) {
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat zip file: %w", err)
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("opening zip archive: %w", err)
	}
	var entries []*zip.File
	for _, e := range zr.File {
		if !e.FileInfo().IsDir() {
			entries = append(entries, e)
		}
	}
	if len(entries) != 1 {
		f.Close()
		return nil, fmt.Errorf("zip archive must contain exactly one file, got %d: %s", len(entries), path)
	}
	rc, err := entries[0].Open()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("opening zip entry: %w", err)
	}
	return composite{r: rc, closers: []io.Closer{rc, f}}, nil
}

// composite pairs a reader with one or more closers that must all be closed.
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

// zstdCloser adapts *zstd.Decoder (whose Close returns nothing) to io.Closer.
type zstdCloser struct{ d *zstd.Decoder }

func (z zstdCloser) Close() error { z.d.Close(); return nil }

// Register adds all gobspect-mcp tools to s.
func Register(s *mcp.Server) {
	registerSchema(s)
	registerTypes(s)
	registerDecode(s)
	registerKeys(s)
	registerTabular(s)
}
