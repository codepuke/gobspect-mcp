package tools_test

import (
	"bytes"
	"context"
	"encoding/gob"
	"strings"
	"testing"

	"github.com/codepuke/gobspect"
	"github.com/codepuke/gobspect-mcp/internal/tools"
	"github.com/codepuke/gobspect/gq"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func callDecode(t *testing.T, in tools.DecodeInput) string {
	t.Helper()
	result, _, err := tools.HandleDecodeForTest(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("callDecode: %v", err)
	}
	return result.Content[0].(*mcp.TextContent).Text
}

func TestHandleDecode_PrettyAll(t *testing.T) {
	out := callDecode(t, tools.DecodeInput{File: fixturePath("simple_struct.gob")})
	if !strings.Contains(out, "alice") {
		t.Errorf("expected 'alice' in output, got: %s", out)
	}
}

func TestHandleDecode_JSON(t *testing.T) {
	out := callDecode(t, tools.DecodeInput{File: fixturePath("simple_struct.gob"), Format: "json"})
	if !strings.Contains(out, `"alice"`) {
		t.Errorf("expected JSON with 'alice', got: %s", out)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("expected JSON object, got: %s", out)
	}
}

func TestHandleDecode_Index(t *testing.T) {
	idx := 1
	out := callDecode(t, tools.DecodeInput{File: fixturePath("multi_value.gob"), Index: &idx})
	if !strings.Contains(out, "alice") {
		t.Errorf("expected index 1 value 'alice', got: %s", out)
	}
}

func TestHandleDecode_Offset(t *testing.T) {
	// multi_value.gob order: charlie(0), alice(1), bob(2); offset 2 yields bob only.
	out := callDecode(t, tools.DecodeInput{File: fixturePath("multi_value.gob"), Offset: 2})
	if !strings.Contains(out, "bob") {
		t.Errorf("expected offset 2 to yield bob (third element), got: %s", out)
	}
	if strings.Contains(out, "charlie") || strings.Contains(out, "alice") {
		t.Errorf("unexpected values before offset: %s", out)
	}
}

func TestHandleDecode_Limit(t *testing.T) {
	out := callDecode(t, tools.DecodeInput{File: fixturePath("multi_value.gob"), Limit: 1})
	if !strings.Contains(out, "charlie") {
		t.Errorf("expected first value charlie, got: %s", out)
	}
	if strings.Contains(out, "alice") || strings.Contains(out, "bob") {
		t.Errorf("unexpected extra values: %s", out)
	}
}

func TestHandleDecode_Sort(t *testing.T) {
	out := callDecode(t, tools.DecodeInput{File: fixturePath("multi_value.gob"), Sort: "Name"})
	alicePos := strings.Index(out, "alice")
	bobPos := strings.Index(out, "bob")
	charliePos := strings.Index(out, "charlie")
	if alicePos < 0 || bobPos < 0 || charliePos < 0 {
		t.Fatalf("missing expected names in output: %s", out)
	}
	if !(alicePos < bobPos && bobPos < charliePos) {
		t.Errorf("expected sorted order alice < bob < charlie, got positions %d %d %d", alicePos, bobPos, charliePos)
	}
}

func TestHandleDecode_SortDesc(t *testing.T) {
	out := callDecode(t, tools.DecodeInput{File: fixturePath("multi_value.gob"), Sort: "Name", SortDesc: true})
	charliePos := strings.Index(out, "charlie")
	alicePos := strings.Index(out, "alice")
	if charliePos < 0 || alicePos < 0 {
		t.Fatalf("missing expected names in output: %s", out)
	}
	if charliePos >= alicePos {
		t.Errorf("expected descending order charlie before alice, got positions %d %d", charliePos, alicePos)
	}
}

func TestHandleDecode_Query(t *testing.T) {
	out := callDecode(t, tools.DecodeInput{File: fixturePath("nested.gob"), Query: ".Inner.Name"})
	if !strings.Contains(out, "inner") {
		t.Errorf("expected 'inner' from nested query, got: %s", out)
	}
}

func TestHandleDecode_NullOnMiss(t *testing.T) {
	out := callDecode(t, tools.DecodeInput{
		File:       fixturePath("simple_struct.gob"),
		Query:      ".NoSuchField",
		NullOnMiss: true,
	})
	if !strings.Contains(out, "null") {
		t.Errorf("expected null on miss, got: %s", out)
	}
}

func TestHandleDecode_PathNotFound(t *testing.T) {
	_, _, err := tools.HandleDecodeForTest(context.Background(), &mcp.CallToolRequest{}, tools.DecodeInput{
		File:  fixturePath("simple_struct.gob"),
		Query: ".NoSuchField",
	})
	if err == nil {
		t.Fatal("expected error when path not found")
	}
}

func TestHandleDecode_TimeFormat(t *testing.T) {
	out := callDecode(t, tools.DecodeInput{
		File:       fixturePath("simple_struct.gob"),
		TimeFormat: "2006-01-02",
	})
	if !strings.Contains(out, "alice") {
		t.Errorf("expected alice in output, got: %s", out)
	}
}

func TestHandleDecode_BadQueryExpression(t *testing.T) {
	_, _, err := tools.HandleDecodeForTest(context.Background(), &mcp.CallToolRequest{}, tools.DecodeInput{
		File:  fixturePath("simple_struct.gob"),
		Query: "[invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid query expression")
	}
}

func TestHandleDecode_RawNonString(t *testing.T) {
	// raw=true on a non-string value (int field) falls through to FormatTo.
	out := callDecode(t, tools.DecodeInput{File: fixturePath("simple_struct.gob"), Query: ".ID", Raw: true})
	if !strings.Contains(out, "1") {
		t.Errorf("expected integer value 1 in raw output, got: %s", out)
	}
}

func TestHandleDecode_SortWithIndex(t *testing.T) {
	idx := 0
	out := callDecode(t, tools.DecodeInput{
		File:  fixturePath("multi_value.gob"),
		Sort:  "Name",
		Index: &idx,
	})
	// Only the first value (charlie) is sorted among itself.
	if !strings.Contains(out, "charlie") {
		t.Errorf("expected charlie (index 0) in sort+index output, got: %s", out)
	}
}

func TestHandleDecode_BadFormat(t *testing.T) {
	_, _, err := tools.HandleDecodeForTest(context.Background(), &mcp.CallToolRequest{}, tools.DecodeInput{
		File:   fixturePath("simple_struct.gob"),
		Format: "xml",
	})
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestHandleDecode_CompactJSON(t *testing.T) {
	out := callDecode(t, tools.DecodeInput{File: fixturePath("simple_struct.gob"), Format: "json", Compact: true})
	// Compact mode: no indentation, single line.
	if strings.Contains(out, "\n  ") {
		t.Errorf("compact JSON should not have indentation, got: %s", out)
	}
	if !strings.Contains(out, `"alice"`) {
		t.Errorf("expected alice in compact output, got: %s", out)
	}
}

func TestHandleDecode_MaxBytes(t *testing.T) {
	// MaxBytes=2 should truncate the bytes field rendering.
	maxBytes := 2
	out := callDecode(t, tools.DecodeInput{
		File:     fixturePath("simple_struct.gob"),
		MaxBytes: &maxBytes,
	})
	// simple_struct.gob has no bytes field, just confirm it doesn't error.
	if !strings.Contains(out, "alice") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestHandleDecode_BadBytesFormat(t *testing.T) {
	_, _, err := tools.HandleDecodeForTest(context.Background(), &mcp.CallToolRequest{}, tools.DecodeInput{
		File:  fixturePath("simple_struct.gob"),
		Bytes: "unknown",
	})
	if err == nil {
		t.Fatal("expected error for unknown bytes format")
	}
}

func TestHandleDecode_SortDropMissing(t *testing.T) {
	// multi_value.gob has only SimpleStruct rows, all with "Name".
	// Using a nonexistent sort key with drop-missing should drop all rows.
	out := callDecode(t, tools.DecodeInput{
		File:            fixturePath("multi_value.gob"),
		Sort:            "NoSuchField",
		SortDropMissing: true,
	})
	// All rows dropped → empty output.
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output after drop-missing all rows, got: %s", out)
	}
}

func TestHandleDecode_Raw(t *testing.T) {
	out := callDecode(t, tools.DecodeInput{File: fixturePath("nested.gob"), Query: ".Inner.Name", Raw: true})
	if strings.Contains(out, `"`) {
		t.Errorf("raw mode should omit quotes, got: %s", out)
	}
	if !strings.Contains(out, "inner") {
		t.Errorf("expected 'inner', got: %s", out)
	}
}

func TestHandleDecode_NegativeIndex(t *testing.T) {
	idx := -1
	_, _, err := tools.HandleDecodeForTest(context.Background(), &mcp.CallToolRequest{},
		tools.DecodeInput{File: fixturePath("multi_value.gob"), Index: &idx})
	if err == nil || !strings.Contains(err.Error(), "index must be non-negative") {
		t.Errorf("expected negative-index error, got: %v", err)
	}
}

// TestHandleDecode_SignedUintFilterLiteral pins the gobspect v0.2.2 fix:
// '+'-prefixed integer literals compare numerically against uint fields.
func TestHandleDecode_SignedUintFilterLiteral(t *testing.T) {
	type counter struct{ N uint }
	out := callDecode(t, tools.DecodeInput{
		Data:  gobBase64(t, counter{N: 5}),
		Query: "[N==+5]",
	})
	if !strings.Contains(out, "5") {
		t.Errorf("expected [N==+5] to match uint 5, got: %s", out)
	}
}

// TestHandleDecode_RawUnwrapsInterfaces covers the raw-mode string check, which
// used to peel a single InterfaceValue layer in this package. It now delegates
// to gq.Render, which unwraps recursively.
//
// The subtests split by reachability: ordinary gob encoding collapses nested
// interfaces, so a stream can only carry the single-wrap shape. The
// doubly-wrapped shape comes from adversarial streams, so it is pinned against
// the renderer directly — and covered end to end by the fuzz targets.
func TestHandleDecode_RawUnwrapsInterfaces(t *testing.T) {
	type holder struct{ V any }

	t.Run("single interface layer through the tool", func(t *testing.T) {
		gob.Register(holder{})

		out := callDecode(t, tools.DecodeInput{
			Data:  gobBase64(t, holder{V: "hello"}),
			Query: ".V",
			Raw:   true,
		})
		assert.Equal(t, "hello\n", out)
	})

	t.Run("nested interface layers", func(t *testing.T) {
		nested := gobspect.InterfaceValue{
			Value: gobspect.InterfaceValue{
				Value: gobspect.StringValue{V: "hello"},
			},
		}

		var buf bytes.Buffer
		require.NoError(t, gq.Render(&buf, nested, gq.RenderOptions{Raw: true}))
		assert.Equal(t, "hello\n", buf.String(), "raw mode must unwrap every interface layer")
	})
}
