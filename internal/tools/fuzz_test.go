// Fuzzing baseline: 2026-08-19. Ran 4m per target, no failures, 7201 corpus
// entries (72.1M execs): FuzzResolve 912, FuzzDecodeTool 1302, FuzzTabularTool
// 1144, FuzzSchemaTool 947, FuzzTypesTool 1050, FuzzKeysTool 1199,
// FuzzQueryAndSortInputs 747.
package tools_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/codepuke/gobspect-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Every tool input is untrusted, so each handler is fuzzed end to end: base64
// blob -> Resolve -> decompress -> decode -> format. Fuzzed bytes always travel
// through the data input, never through file, so fuzzing never touches the
// caller's filesystem — the same reason gobspect's own gq fuzzer excludes
// -file and -diff.

const (
	fuzzReadLimit   int64 = 1 << 20
	fuzzOutputLimit       = 1 << 16
)

// fuzzSeeds returns the gob fixtures plus a compressed encoding of each, plus
// deliberately malformed archives — including the malformed-zip cases the
// deleted openZip helper never had coverage for.
func fuzzSeeds(tb testing.TB) [][]byte {
	tb.Helper()

	paths, err := filepath.Glob(filepath.Join("testdata", "*.gob"))
	if err != nil {
		tb.Fatalf("globbing fixtures: %v", err)
	}
	if len(paths) == 0 {
		tb.Fatal("no gob fixtures found in testdata")
	}

	var seeds [][]byte
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			tb.Fatalf("reading %s: %v", p, err)
		}
		seeds = append(seeds, raw,
			gzipToBytes(tb, raw),
			zstdToBytes(tb, raw),
			xzToBytes(tb, raw),
			bzip2ToBytes(tb, raw),
			zipToBytes(tb, raw),
		)
	}

	seeds = append(seeds,
		nil,
		[]byte("PK\x03\x04truncated"),
		[]byte("PK\x05\x06"),
		twoEntryZip(tb),
		[]byte{0x1f, 0x8b, 0xff, 0xff},
		[]byte{0x28, 0xb5, 0x2f, 0xfd, 0x00},
		[]byte{0xfd, '7', 'z', 'X', 'Z', 0x00, 0x01},
		[]byte("BZh9garbage"),
	)
	return seeds
}

func addSeeds(f *testing.F, extra ...any) {
	for _, s := range fuzzSeeds(f) {
		args := append([]any{base64.StdEncoding.EncodeToString(s)}, extra...)
		f.Add(args...)
	}
}

func FuzzResolve(f *testing.F) {
	for _, s := range fuzzSeeds(f) {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		enc := base64.StdEncoding.EncodeToString(data)

		// Full read.
		r, err := tools.Resolve(enc, "", fuzzReadLimit)
		if err == nil {
			_, _ = io.Copy(io.Discard, r)
			if err := r.Close(); err != nil {
				t.Logf("close after full read: %v", err)
			}
		}

		// Partial read: Close must be safe here too.
		r, err = tools.Resolve(enc, "", fuzzReadLimit)
		if err == nil {
			buf := make([]byte, 3)
			_, _ = r.Read(buf)
			if err := r.Close(); err != nil {
				t.Logf("close after partial read: %v", err)
			}
		}
	})
}

func FuzzDecodeTool(f *testing.F) {
	addSeeds(f, uint8(0), "")

	formats := []string{"", "pretty", "json"}

	f.Fuzz(func(t *testing.T, data string, knob uint8, query string) {
		idx := int(knob % 4)
		in := tools.DecodeInput{
			Data:        data,
			Query:       query,
			Format:      formats[int(knob)%len(formats)],
			Offset:      int(knob % 3),
			Limit:       int(knob % 5),
			Raw:         knob%2 == 0,
			Compact:     knob%3 == 0,
			NullOnMiss:  knob%5 == 0,
			ReadLimit:   ptrInt64(fuzzReadLimit),
			OutputLimit: ptrInt(fuzzOutputLimit),
		}
		if knob%7 == 0 {
			in.Index = &idx
		}
		if knob%11 == 0 {
			in.Sort = "Name"
			in.SortDesc = knob%2 == 0
		}

		result, _, err := tools.HandleDecodeForTest(context.Background(), &mcp.CallToolRequest{}, in)
		if err != nil {
			return
		}
		assertWithinOutputLimit(t, result)
	})
}

func FuzzTabularTool(f *testing.F) {
	addSeeds(f, uint8(0), "")

	formats := []string{"", "csv", "tsv"}
	heteros := []string{"", "first", "reject", "union", "partition"}

	f.Fuzz(func(t *testing.T, data string, knob uint8, query string) {
		in := tools.TabularInput{
			Data:        data,
			Query:       query,
			Format:      formats[int(knob)%len(formats)],
			Hetero:      heteros[int(knob)%len(heteros)],
			Offset:      int(knob % 3),
			Limit:       int(knob % 5),
			NoHeaders:   knob%2 == 0,
			ReadLimit:   ptrInt64(fuzzReadLimit),
			OutputLimit: ptrInt(fuzzOutputLimit),
		}
		if knob%11 == 0 {
			in.Sort = "Name"
		}

		result, _, err := tools.HandleTabularForTest(context.Background(), &mcp.CallToolRequest{}, in)
		if err != nil {
			return
		}
		assertWithinOutputLimit(t, result)
	})
}

func FuzzSchemaTool(f *testing.F) {
	addSeeds(f)

	f.Fuzz(func(t *testing.T, data string) {
		result, _, err := tools.HandleSchemaForTest(context.Background(), &mcp.CallToolRequest{},
			tools.SchemaInput{Data: data, ReadLimit: ptrInt64(fuzzReadLimit), OutputLimit: ptrInt(fuzzOutputLimit)})
		if err != nil {
			return
		}
		assertWithinOutputLimit(t, result)
	})
}

func FuzzTypesTool(f *testing.F) {
	addSeeds(f)

	f.Fuzz(func(t *testing.T, data string) {
		result, _, err := tools.HandleTypesForTest(context.Background(), &mcp.CallToolRequest{},
			tools.TypesInput{Data: data, ReadLimit: ptrInt64(fuzzReadLimit), OutputLimit: ptrInt(fuzzOutputLimit)})
		if err != nil {
			return
		}
		text := resultText(t, result)
		if !json.Valid([]byte(text)) {
			t.Fatalf("gob_types returned invalid JSON: %q", text)
		}
	})
}

func FuzzKeysTool(f *testing.F) {
	addSeeds(f, "")

	f.Fuzz(func(t *testing.T, data string, query string) {
		result, _, err := tools.HandleKeysForTest(context.Background(), &mcp.CallToolRequest{},
			tools.KeysInput{Data: data, Query: query,
				ReadLimit: ptrInt64(fuzzReadLimit), OutputLimit: ptrInt(fuzzOutputLimit)})
		if err != nil {
			return
		}
		text := resultText(t, result)
		if !json.Valid([]byte(text)) {
			t.Fatalf("gob_keys returned invalid JSON: %q", text)
		}
	})
}

// FuzzQueryAndSortInputs drives the two parser-backed input fields against a
// fixed, valid stream. query.Parse and sortval.ParseSortSpec must surface
// errors as tool errors — the convenience wrappers panic, so a panic here means
// a handler reached for the wrong one.
func FuzzQueryAndSortInputs(f *testing.F) {
	for _, q := range []string{"", ".Name", ".*", "..V", ".Items.*", "[N==+5]", ".[", "***"} {
		for _, s := range []string{"", "Name", "Name:desc", "ID,Name", ":", ",,"} {
			f.Add(q, s)
		}
	}

	fixture, err := os.ReadFile(filepath.Join("testdata", "multi_value.gob"))
	if err != nil {
		f.Fatalf("reading fixture: %v", err)
	}
	data := base64.StdEncoding.EncodeToString(fixture)

	f.Fuzz(func(t *testing.T, query, sort string) {
		_, _, _ = tools.HandleDecodeForTest(context.Background(), &mcp.CallToolRequest{},
			tools.DecodeInput{Data: data, Query: query, Sort: sort,
				ReadLimit: ptrInt64(fuzzReadLimit), OutputLimit: ptrInt(fuzzOutputLimit)})

		_, _, _ = tools.HandleTabularForTest(context.Background(), &mcp.CallToolRequest{},
			tools.TabularInput{Data: data, Query: query, Sort: sort,
				ReadLimit: ptrInt64(fuzzReadLimit), OutputLimit: ptrInt(fuzzOutputLimit)})

		_, _, _ = tools.HandleKeysForTest(context.Background(), &mcp.CallToolRequest{},
			tools.KeysInput{Data: data, Query: query,
				ReadLimit: ptrInt64(fuzzReadLimit), OutputLimit: ptrInt(fuzzOutputLimit)})
	})
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatal("handler returned no content")
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("unexpected content type %T", result.Content[0])
	}
	return tc.Text
}

// The response must respect the caller's output limit, allowing for the
// truncation notice appended past it.
func assertWithinOutputLimit(t *testing.T, result *mcp.CallToolResult) {
	t.Helper()
	text := resultText(t, result)
	max := fuzzOutputLimit + len(truncationNoticeFor(fuzzOutputLimit))
	if len(text) > max {
		t.Fatalf("response of %d bytes exceeds output limit %d (+notice)", len(text), fuzzOutputLimit)
	}
}

func truncationNoticeFor(limit int) string {
	return fmt.Sprintf("... output truncated at %d bytes; narrow the query, lower limit, or raise output_limit\n", limit)
}

func twoEntryZip(tb testing.TB) []byte {
	tb.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"a.gob", "b.gob"} {
		w, err := zw.Create(name)
		if err != nil {
			tb.Fatalf("creating zip entry: %v", err)
		}
		if _, err := w.Write([]byte("data")); err != nil {
			tb.Fatalf("writing zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		tb.Fatalf("closing zip: %v", err)
	}
	return buf.Bytes()
}
