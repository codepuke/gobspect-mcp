package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/codepuke/gobspect-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHandleTypes_File(t *testing.T) {
	in := tools.TypesInput{File: fixturePath("simple_struct.gob")}
	result, _, err := tools.HandleTypesForTest(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "SimpleStruct") {
		t.Errorf("expected type name in types output, got: %s", text)
	}
	// Must be valid JSON array.
	if !strings.HasPrefix(strings.TrimSpace(text), "[") {
		t.Errorf("expected JSON array, got: %s", text)
	}
}

func TestHandleTypes_Base64(t *testing.T) {
	in := tools.TypesInput{Data: fixtureBase64(t, "simple_struct.gob")}
	result, _, err := tools.HandleTypesForTest(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "SimpleStruct") {
		t.Errorf("expected type name in types output, got: %s", text)
	}
}

func TestHandleTypes_TimeFormat(t *testing.T) {
	in := tools.TypesInput{File: fixturePath("simple_struct.gob"), TimeFormat: "2006-01-02"}
	result, _, err := tools.HandleTypesForTest(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "SimpleStruct") {
		t.Errorf("expected type name in types output, got: %s", text)
	}
}

func TestHandleTypes_MultiValue(t *testing.T) {
	in := tools.TypesInput{File: fixturePath("multi_value.gob")}
	result, _, err := tools.HandleTypesForTest(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "SimpleStruct") {
		t.Errorf("expected SimpleStruct in types output, got: %s", text)
	}
}
