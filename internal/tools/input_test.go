package tools_test

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"io"
	"testing"

	"github.com/codepuke/gobspect-mcp/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_Base64Std(t *testing.T) {
	b := gobEncode(t, 42)
	r, err := tools.Resolve(base64.StdEncoding.EncodeToString(b), "")
	require.NoError(t, err)
	defer r.Close()
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, b, got)
}

func TestResolve_Base64RawFallback(t *testing.T) {
	b := gobEncode(t, "hello")
	// RawStdEncoding omits padding '=' characters.
	raw := base64.RawStdEncoding.EncodeToString(b)
	r, err := tools.Resolve(raw, "")
	require.NoError(t, err)
	defer r.Close()
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, b, got)
}

func TestResolve_InvalidBase64(t *testing.T) {
	_, err := tools.Resolve("!!!not-base64!!!", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding base64")
}

func TestResolve_BothProvided(t *testing.T) {
	_, err := tools.Resolve("dGVzdA==", "somefile.gob")
	require.Error(t, err)
}

func TestResolve_NeitherProvided(t *testing.T) {
	_, err := tools.Resolve("", "")
	require.Error(t, err)
}

func TestResolve_File(t *testing.T) {
	r, err := tools.Resolve("", fixturePath("simple_struct.gob"))
	require.NoError(t, err)
	defer r.Close()
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.NotEmpty(t, b)
}

func TestResolve_FileNotFound(t *testing.T) {
	_, err := tools.Resolve("", "/nonexistent/path/to/file.gob")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "opening file")
}

func gobEncode(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(v))
	return buf.Bytes()
}
