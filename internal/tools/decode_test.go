package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/codepuke/gobspect-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
