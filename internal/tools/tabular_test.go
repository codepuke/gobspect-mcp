package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/codepuke/gobspect-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func callTabular(t *testing.T, in tools.TabularInput) string {
	t.Helper()
	result, _, err := tools.HandleTabularForTest(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("callTabular: %v", err)
	}
	return result.Content[0].(*mcp.TextContent).Text
}

func TestHandleTabular_CSV(t *testing.T) {
	out := callTabular(t, tools.TabularInput{File: fixturePath("multi_value.gob")})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 4 { // header + 3 rows
		t.Fatalf("expected at least 4 lines, got %d:\n%s", len(lines), out)
	}
	if !strings.Contains(lines[0], "ID") || !strings.Contains(lines[0], "Name") {
		t.Errorf("expected header with ID and Name, got: %s", lines[0])
	}
	if !strings.Contains(out, "alice") || !strings.Contains(out, "bob") || !strings.Contains(out, "charlie") {
		t.Errorf("expected all names in output, got: %s", out)
	}
}

func TestHandleTabular_TSV(t *testing.T) {
	out := callTabular(t, tools.TabularInput{File: fixturePath("multi_value.gob"), Format: "tsv"})
	if !strings.Contains(out, "\t") {
		t.Errorf("expected tab-separated output, got: %s", out)
	}
}

func TestHandleTabular_NoHeaders(t *testing.T) {
	out := callTabular(t, tools.TabularInput{File: fixturePath("multi_value.gob"), NoHeaders: true})
	if strings.Contains(out, "ID") || strings.Contains(out, "Name") {
		t.Errorf("expected no header row, got: %s", out)
	}
}

func TestHandleTabular_Sort(t *testing.T) {
	out := callTabular(t, tools.TabularInput{File: fixturePath("multi_value.gob"), Sort: "Name"})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// lines[0] = header; data lines should be alice, bob, charlie
	if len(lines) < 4 {
		t.Fatalf("too few lines: %s", out)
	}
	if !strings.HasPrefix(lines[1], "1,") { // alice has ID=1
		t.Errorf("expected alice first after sort, got: %s", lines[1])
	}
}

func TestHandleTabular_Limit(t *testing.T) {
	out := callTabular(t, tools.TabularInput{File: fixturePath("multi_value.gob"), Limit: 1})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 { // header + 1 data row
		t.Errorf("expected 2 lines (header+1 data), got %d:\n%s", len(lines), out)
	}
}

func TestHandleTabular_HeteroReject(t *testing.T) {
	_, _, err := tools.HandleTabularForTest(context.Background(), &mcp.CallToolRequest{}, tools.TabularInput{
		File:   fixturePath("hetero.gob"),
		Hetero: "reject",
	})
	if err == nil {
		t.Fatal("expected error for reject mode with hetero stream")
	}
}

func TestHandleTabular_HeteroFirstWins(t *testing.T) {
	out := callTabular(t, tools.TabularInput{File: fixturePath("hetero.gob"), Hetero: "first"})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Only rows matching the first type should appear.
	if len(lines) < 2 {
		t.Fatalf("expected at least header + 1 row, got: %s", out)
	}
}

func TestHandleTabular_BadFormat(t *testing.T) {
	_, _, err := tools.HandleTabularForTest(context.Background(), &mcp.CallToolRequest{}, tools.TabularInput{
		File:   fixturePath("simple_struct.gob"),
		Format: "json",
	})
	if err == nil {
		t.Fatal("expected error for non-csv/tsv format")
	}
}

func TestHandleTabular_Query(t *testing.T) {
	out := callTabular(t, tools.TabularInput{File: fixturePath("nested.gob"), Query: ".Inner"})
	if !strings.Contains(out, "ID") || !strings.Contains(out, "Name") {
		t.Errorf("expected inner struct fields after query, got: %s", out)
	}
}
