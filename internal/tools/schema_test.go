package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/codepuke/gobspect-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHandleSchema_File(t *testing.T) {
	req := &mcp.CallToolRequest{}
	in := tools.SchemaInput{File: fixturePath("simple_struct.gob")}

	result, _, err := callSchema(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "SimpleStruct") {
		t.Errorf("expected type name in schema output, got: %s", text)
	}
	_ = req
}

func TestHandleSchema_Base64(t *testing.T) {
	in := tools.SchemaInput{Data: fixtureBase64(t, "simple_struct.gob")}
	result, _, err := callSchema(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "SimpleStruct") {
		t.Errorf("expected type name in schema output, got: %s", text)
	}
}

func TestHandleSchema_BothInputs(t *testing.T) {
	in := tools.SchemaInput{Data: "dGVzdA==", File: "somefile"}
	_, _, err := callSchema(in)
	if err == nil {
		t.Fatal("expected error when both data and file provided")
	}
}

func TestHandleSchema_NoInput(t *testing.T) {
	in := tools.SchemaInput{}
	_, _, err := callSchema(in)
	if err == nil {
		t.Fatal("expected error when neither data nor file provided")
	}
}

func TestHandleSchema_TimeFormat(t *testing.T) {
	in := tools.SchemaInput{File: fixturePath("simple_struct.gob"), TimeFormat: "2006-01-02"}
	result, _, err := tools.HandleSchemaForTest(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "SimpleStruct") {
		t.Errorf("expected type name in schema output, got: %s", text)
	}
}

// callSchema invokes the schema handler directly via the exported type.
func callSchema(in tools.SchemaInput) (*mcp.CallToolResult, any, error) {
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.Register(s)
	// Use reflection-free approach: call the underlying handler by rebuilding it.
	// Since handlers are not exported, we call through the test-only exported shim.
	return tools.HandleSchemaForTest(context.Background(), &mcp.CallToolRequest{}, in)
}
