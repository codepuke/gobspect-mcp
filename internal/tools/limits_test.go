package tools_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"strings"
	"testing"

	"github.com/codepuke/gobspect-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptrInt(n int) *int       { return &n }
func ptrInt64(n int64) *int64 { return &n }

func TestLimits_Validation(t *testing.T) {
	cases := []struct {
		name    string
		in      tools.DecodeInput
		wantErr string
	}{
		{"read limit zero", tools.DecodeInput{ReadLimit: ptrInt64(0)}, "read_limit must be positive"},
		{"read limit negative", tools.DecodeInput{ReadLimit: ptrInt64(-1)}, "read_limit must be positive"},
		{"read limit over max", tools.DecodeInput{ReadLimit: ptrInt64(1<<30 + 1)}, "exceeds the maximum"},
		{"output limit zero", tools.DecodeInput{OutputLimit: ptrInt(0)}, "output_limit must be positive"},
		{"output limit negative", tools.DecodeInput{OutputLimit: ptrInt(-5)}, "output_limit must be positive"},
		{"output limit over max", tools.DecodeInput{OutputLimit: ptrInt(16<<20 + 1)}, "exceeds the maximum"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.in
			in.File = fixturePath("simple_struct.gob")
			_, _, err := tools.HandleDecodeForTest(context.Background(), &mcp.CallToolRequest{}, in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestLimits_DefaultsAllowNormalInput(t *testing.T) {
	out := callDecode(t, tools.DecodeInput{File: fixturePath("multi_value.gob")})
	assert.Contains(t, out, "alice")
	assert.NotContains(t, out, "truncated")
}

// The read limit applies to the decompressed side, so a compression bomb fails
// instead of expanding into memory. The payload is a valid, highly compressible
// gob stream, so decoding genuinely proceeds until the cap stops it.
func TestLimits_ReadLimitStopsDecompressionBomb(t *testing.T) {
	type row struct {
		ID   int
		Name string
	}
	var raw bytes.Buffer
	enc := gob.NewEncoder(&raw)
	for i := range 200_000 {
		require.NoError(t, enc.Encode(row{ID: i, Name: strings.Repeat("z", 128)}))
	}

	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, err := zw.Write(raw.Bytes())
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	t.Logf("bomb: %d compressed bytes -> %d decompressed", gz.Len(), raw.Len())

	data := base64.StdEncoding.EncodeToString(gz.Bytes())

	// gob_types drains the whole stream but emits only a small type table, so
	// nothing but the read limit can stop it.
	_, _, err = tools.HandleTypesForTest(context.Background(), &mcp.CallToolRequest{},
		tools.TypesInput{Data: data, ReadLimit: ptrInt64(1 << 20)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limit of 1048576",
		"the read limit, not a parse failure, must be what stops the bomb")

	// The same bomb through gob_decode returns a bounded, truncated response
	// rather than a multi-megabyte one.
	out := callDecode(t, tools.DecodeInput{
		Data:        data,
		ReadLimit:   ptrInt64(1 << 20),
		OutputLimit: ptrInt(64 << 10),
	})
	assert.Contains(t, out, "output truncated")
	assert.LessOrEqual(t, len(out), 64<<10+len("... output truncated at 65536 bytes; narrow the query, lower limit, or raise output_limit\n"))
}

// Decode truncates between whole values and says so.
func TestLimits_DecodeTruncatesAtRecordBoundary(t *testing.T) {
	type row struct {
		ID   int
		Name string
	}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for i := range 200 {
		require.NoError(t, enc.Encode(row{ID: i, Name: strings.Repeat("x", 64)}))
	}

	out := callDecode(t, tools.DecodeInput{
		Data:        base64.StdEncoding.EncodeToString(buf.Bytes()),
		Format:      "json",
		Compact:     true,
		OutputLimit: ptrInt(2000),
	})

	assert.Contains(t, out, "output truncated at 2000 bytes")

	// Every line before the notice must still be a complete JSON document.
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	require.Greater(t, len(lines), 1)
	for _, ln := range lines[:len(lines)-1] {
		assert.True(t, json.Valid([]byte(ln)), "truncated output left an incomplete record: %q", ln)
	}
}

func TestLimits_TabularTruncatesAtLineBoundary(t *testing.T) {
	type row struct {
		ID   int
		Name string
	}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for i := range 500 {
		require.NoError(t, enc.Encode(row{ID: i, Name: strings.Repeat("y", 32)}))
	}

	out := callTabular(t, tools.TabularInput{
		Data:        base64.StdEncoding.EncodeToString(buf.Bytes()),
		OutputLimit: ptrInt(1500),
	})

	assert.Contains(t, out, "output truncated at 1500 bytes")
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	for _, ln := range lines[:len(lines)-1] {
		assert.Equal(t, 2, len(strings.Split(ln, ",")), "truncated output left a partial row: %q", ln)
	}
}

// types and keys return one JSON document, so they refuse rather than truncate.
func TestLimits_JSONToolsRefuseRatherThanTruncate(t *testing.T) {
	t.Run("types", func(t *testing.T) {
		_, _, err := tools.HandleTypesForTest(context.Background(), &mcp.CallToolRequest{},
			tools.TypesInput{File: fixturePath("nested.gob"), OutputLimit: ptrInt(16)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds output_limit")
	})

	t.Run("keys", func(t *testing.T) {
		_, _, err := tools.HandleKeysForTest(context.Background(), &mcp.CallToolRequest{},
			tools.KeysInput{File: fixturePath("simple_struct.gob"), OutputLimit: ptrInt(2)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds output_limit")
	})
}

func TestLimits_SchemaTruncates(t *testing.T) {
	result, _, err := tools.HandleSchemaForTest(context.Background(), &mcp.CallToolRequest{},
		tools.SchemaInput{File: fixturePath("nested.gob"), OutputLimit: ptrInt(20)})
	require.NoError(t, err)
	out := result.Content[0].(*mcp.TextContent).Text
	assert.Contains(t, out, "output truncated at 20 bytes")
}
