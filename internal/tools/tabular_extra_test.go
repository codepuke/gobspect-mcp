package tools_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"strings"
	"testing"

	"github.com/codepuke/gobspect"
	"github.com/codepuke/gobspect-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

// encodeGob builds a multi-value gob stream from vals.
func encodeGob(t *testing.T, vals ...any) string {
	t.Helper()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, v := range vals {
		if err := enc.Encode(v); err != nil {
			t.Fatalf("encodeGob: %v", err)
		}
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

type TypeA struct{ X int }
type TypeB struct {
	X int
	Y string
}

func TestHandleTabular_HeteroUnion(t *testing.T) {
	gob.Register(TypeA{})
	gob.Register(TypeB{})
	data := encodeGob(t, TypeA{X: 1}, TypeB{X: 2, Y: "extra"})
	out := callTabular(t, tools.TabularInput{Data: data, Hetero: "union"})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Union grows headers: first type has X; second type adds Y.
	// The re-emitted header (after union grows it) should have both.
	found := false
	for _, l := range lines {
		if strings.Contains(l, "Y") {
			found = true
		}
	}
	assert.True(t, found, "union mode should include column Y from second type:\n%s", out)
}

func TestHandleTabular_HeteroPartition(t *testing.T) {
	gob.Register(TypeA{})
	gob.Register(TypeB{})
	data := encodeGob(t, TypeA{X: 1}, TypeB{X: 2, Y: "extra"})
	out := callTabular(t, tools.TabularInput{Data: data, Hetero: "partition"})
	// Partition emits a blank line between partitions.
	if !strings.Contains(out, "\n\n") {
		t.Errorf("partition mode should emit a blank line between sections:\n%s", out)
	}
	// Both X headers should appear.
	if strings.Count(out, "X") < 2 {
		t.Errorf("partition mode should emit a new header for each type:\n%s", out)
	}
}

func TestHandleTabular_ScalarRow(t *testing.T) {
	// Query .X on TypeA returns a scalar (IntValue), which goes through writeScalarRow.
	gob.Register(TypeA{})
	data := encodeGob(t, TypeA{X: 99})
	out := callTabular(t, tools.TabularInput{Data: data, Query: ".X"})
	assert.Contains(t, out, "value", "scalar header should be 'value'")
	assert.Contains(t, out, "99")
}

// TestTabularCellString exercises the tabularCellString function across all branches.
func TestTabularCellString(t *testing.T) {
	fn := tools.TabularCellStringForTest
	assert.Equal(t, "hello", fn(gobspect.StringValue{V: "hello"}))
	assert.Equal(t, "42", fn(gobspect.IntValue{V: 42}))
	assert.Equal(t, "7", fn(gobspect.UintValue{V: 7}))
	assert.Equal(t, "3.14", fn(gobspect.FloatValue{V: 3.14}))
	assert.Equal(t, "true", fn(gobspect.BoolValue{V: true}))
	assert.Equal(t, "false", fn(gobspect.BoolValue{V: false}))
	assert.Equal(t, "", fn(gobspect.NilValue{}))
	assert.Equal(t, "deadbeef", fn(gobspect.BytesValue{V: []byte{0xde, 0xad, 0xbe, 0xef}}))
	assert.Equal(t, "(opaque)", fn(gobspect.OpaqueValue{}))
	assert.Equal(t, "decoded", fn(gobspect.OpaqueValue{Decoded: "decoded"}))
	assert.Equal(t, "hello", fn(gobspect.InterfaceValue{Value: gobspect.StringValue{V: "hello"}}))
	assert.Equal(t, "(struct)", fn(gobspect.StructValue{}))
	assert.Equal(t, "(slice)", fn(gobspect.SliceValue{}))
	assert.Equal(t, "(array)", fn(gobspect.ArrayValue{}))
	assert.Equal(t, "(map)", fn(gobspect.MapValue{}))
	// Complex value — positive imaginary.
	assert.Equal(t, "(1+2i)", fn(gobspect.ComplexValue{Real: 1, Imag: 2}))
	// Complex value — negative imaginary.
	assert.Equal(t, "(1-2i)", fn(gobspect.ComplexValue{Real: 1, Imag: -2}))
}

func TestHandleTabular_BytesFormat(t *testing.T) {
	type WithBytes struct{ Data []byte }
	gob.Register(WithBytes{})
	data := encodeGob(t, WithBytes{Data: []byte{0xca, 0xfe}})

	outHex := callTabular(t, tools.TabularInput{Data: data, Bytes: "hex"})
	assert.Contains(t, outHex, "cafe")

	outB64 := callTabular(t, tools.TabularInput{Data: data, Bytes: "base64"})
	// base64 of 0xca,0xfe is "yv4="
	assert.Contains(t, outB64, "yv4=")
}

func TestHandleTabular_ParseHeteroMode(t *testing.T) {
	for _, mode := range []string{"first", "reject", "union", "partition", ""} {
		_, _, err := tools.HandleTabularForTest(context.Background(), &mcp.CallToolRequest{}, tools.TabularInput{
			File:   fixturePath("simple_struct.gob"),
			Hetero: mode,
		})
		// single-type stream never triggers hetero logic; all modes should succeed
		if err != nil {
			t.Errorf("mode %q: unexpected error: %v", mode, err)
		}
	}
	_, _, err := tools.HandleTabularForTest(context.Background(), &mcp.CallToolRequest{}, tools.TabularInput{
		File:   fixturePath("simple_struct.gob"),
		Hetero: "badvalue",
	})
	assert.Error(t, err)
}

func TestHandleTabular_Offset(t *testing.T) {
	// multi_value.gob: charlie(0), alice(1), bob(2); offset 2 yields only bob.
	out := callTabular(t, tools.TabularInput{File: fixturePath("multi_value.gob"), Offset: 2})
	assert.Contains(t, out, "bob")
	assert.NotContains(t, out, "charlie")
	assert.NotContains(t, out, "alice")
}

func TestHandleTabular_InterfaceValue(t *testing.T) {
	// Querying a field wrapped in an interface goes through WriteValue's InterfaceValue branch.
	type Holder struct{ Pet any }
	type Dog struct{ Name string }
	gob.Register(Dog{})
	gob.Register(Holder{})
	data := encodeGob(t, Holder{Pet: Dog{Name: "Rex"}})
	out := callTabular(t, tools.TabularInput{Data: data, Query: ".Pet"})
	// Dog is the concrete type; its Name field should appear.
	assert.Contains(t, out, "Rex")
}

func TestHandleTabular_ScalarNoHeaders(t *testing.T) {
	data := encodeGob(t, "hello")
	out := callTabular(t, tools.TabularInput{Data: data, NoHeaders: true})
	assert.NotContains(t, out, "value")
	assert.Contains(t, out, "hello")
}

func TestHandleTabular_SecondScalarRow(t *testing.T) {
	// Two scalar values → two data rows under the same "value" header.
	data := encodeGob(t, "first", "second")
	out := callTabular(t, tools.TabularInput{Data: data})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	assert.Len(t, lines, 3, "expected header + 2 scalar rows:\n%s", out)
	assert.Equal(t, "value", lines[0])
	assert.Contains(t, out, "first")
	assert.Contains(t, out, "second")
}

func TestHandleTabular_Projection(t *testing.T) {
	// A projection query like ".ID,Name" returns a ProjectionTypeName struct.
	out := callTabular(t, tools.TabularInput{File: fixturePath("multi_value.gob"), Query: ".ID,Name"})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Header row from projection should list projected field names.
	assert.True(t, len(lines) >= 4, "expected header + 3 data rows, got:\n%s", out)
	assert.Contains(t, lines[0], "ID")
}

func TestHandleTabular_SortWithIndex(t *testing.T) {
	idx := 0
	out := callTabular(t, tools.TabularInput{
		File:  fixturePath("multi_value.gob"),
		Sort:  "Name",
		Index: &idx,
	})
	// Only the first value (charlie) is included.
	assert.Contains(t, out, "charlie")
	assert.NotContains(t, out, "alice")
}

func TestHandleTabular_BadBytesFormat(t *testing.T) {
	_, _, err := tools.HandleTabularForTest(context.Background(), &mcp.CallToolRequest{}, tools.TabularInput{
		File:  fixturePath("simple_struct.gob"),
		Bytes: "badformat",
	})
	assert.Error(t, err)
}

func TestHandleTabular_TimeFormat(t *testing.T) {
	out := callTabular(t, tools.TabularInput{
		File:       fixturePath("simple_struct.gob"),
		TimeFormat: "2006-01-02",
	})
	assert.Contains(t, out, "alice")
}

func TestHandleTabular_BadQueryExpression(t *testing.T) {
	_, _, err := tools.HandleTabularForTest(context.Background(), &mcp.CallToolRequest{}, tools.TabularInput{
		File:  fixturePath("simple_struct.gob"),
		Query: "[invalid",
	})
	assert.Error(t, err)
}
