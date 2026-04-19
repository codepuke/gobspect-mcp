package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/codepuke/gobspect-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func callKeys(t *testing.T, in tools.KeysInput) string {
	t.Helper()
	result, _, err := tools.HandleKeysForTest(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("callKeys: %v", err)
	}
	return result.Content[0].(*mcp.TextContent).Text
}

func TestHandleKeys_Struct(t *testing.T) {
	out := callKeys(t, tools.KeysInput{File: fixturePath("simple_struct.gob")})
	if !strings.Contains(out, "ID") || !strings.Contains(out, "Name") {
		t.Errorf("expected field names ID and Name, got: %s", out)
	}
}

func TestHandleKeys_Slice(t *testing.T) {
	out := callKeys(t, tools.KeysInput{File: fixturePath("slice_value.gob")})
	if !strings.Contains(out, "0") || !strings.Contains(out, "1") || !strings.Contains(out, "2") {
		t.Errorf("expected index strings 0,1,2, got: %s", out)
	}
}

func TestHandleKeys_Map(t *testing.T) {
	out := callKeys(t, tools.KeysInput{File: fixturePath("map_value.gob")})
	if !strings.Contains(out, "x") || !strings.Contains(out, "y") {
		t.Errorf("expected map keys x and y, got: %s", out)
	}
}

func TestHandleKeys_NestedPath(t *testing.T) {
	out := callKeys(t, tools.KeysInput{File: fixturePath("nested.gob"), Query: ".Inner"})
	if !strings.Contains(out, "ID") || !strings.Contains(out, "Name") {
		t.Errorf("expected inner struct fields, got: %s", out)
	}
}

func TestHandleKeys_PathNotFound(t *testing.T) {
	_, _, err := tools.HandleKeysForTest(context.Background(), &mcp.CallToolRequest{}, tools.KeysInput{
		File:  fixturePath("simple_struct.gob"),
		Query: ".NoSuchField",
	})
	if err == nil {
		t.Fatal("expected error when path not found")
	}
}

func TestHandleKeys_TimeFormat(t *testing.T) {
	out := callKeys(t, tools.KeysInput{File: fixturePath("simple_struct.gob"), TimeFormat: "2006-01-02"})
	if !strings.Contains(out, "ID") {
		t.Errorf("expected field names, got: %s", out)
	}
}

func TestHandleKeys_IndexOutOfBounds(t *testing.T) {
	idx := 99
	_, _, err := tools.HandleKeysForTest(context.Background(), &mcp.CallToolRequest{}, tools.KeysInput{
		File:  fixturePath("simple_struct.gob"),
		Index: &idx,
	})
	if err == nil {
		t.Fatal("expected error for out-of-bounds index")
	}
}

func TestHandleKeys_Index(t *testing.T) {
	idx := 1
	out := callKeys(t, tools.KeysInput{File: fixturePath("multi_value.gob"), Index: &idx})
	// multi_value.gob index 1 is SimpleStruct{ID:1, Name:"alice"}
	if !strings.Contains(out, "ID") || !strings.Contains(out, "Name") {
		t.Errorf("expected SimpleStruct field names, got: %s", out)
	}
}
