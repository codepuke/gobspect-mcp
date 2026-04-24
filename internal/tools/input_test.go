package tools_test

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/gob"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/codepuke/gobspect-mcp/internal/tools"
	dsnetbz2 "github.com/dsnet/compress/bzip2"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
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

func TestResolve_CompressedFiles(t *testing.T) {
	raw, err := os.ReadFile(fixturePath("simple_struct.gob"))
	require.NoError(t, err)

	cases := []struct {
		name    string
		ext     string
		compress func(t *testing.T, path string, raw []byte)
	}{
		{"gzip lowercase", ".gz", writeGzip},
		{"gzip long ext", ".gzip", writeGzip},
		{"gzip uppercase", ".GZ", writeGzip},
		{"zstd short", ".zst", writeZstd},
		{"zstd long", ".zstd", writeZstd},
		{"bzip2", ".bz2", writeBzip2},
		{"xz", ".xz", writeXz},
		{"zip single entry", ".zip", writeZipSingle},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "data.gob"+tc.ext)
			tc.compress(t, path, raw)

			r, err := tools.Resolve("", path)
			require.NoError(t, err)
			defer r.Close()

			got, err := io.ReadAll(r)
			require.NoError(t, err)
			assert.Equal(t, raw, got)
		})
	}
}

func TestResolve_UnknownExtensionTreatedAsRaw(t *testing.T) {
	raw, err := os.ReadFile(fixturePath("simple_struct.gob"))
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "data.weird")
	require.NoError(t, os.WriteFile(path, raw, 0o644))

	r, err := tools.Resolve("", path)
	require.NoError(t, err)
	defer r.Close()

	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, raw, got)
}

func TestResolve_CorruptGzip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bogus.gob.gz")
	require.NoError(t, os.WriteFile(path, []byte("not a gzip stream"), 0o644))

	_, err := tools.Resolve("", path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gzip")
}

func TestResolve_CorruptZstd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bogus.gob.zst")
	require.NoError(t, os.WriteFile(path, []byte("not a zstd stream"), 0o644))

	// klauspost/compress validates lazily — the error may surface on the
	// first read rather than on NewReader.
	r, err := tools.Resolve("", path)
	if err == nil {
		_, err = io.ReadAll(r)
		r.Close()
	}
	require.Error(t, err)
}

func TestResolve_CorruptXz(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bogus.gob.xz")
	require.NoError(t, os.WriteFile(path, []byte("not an xz stream"), 0o644))

	_, err := tools.Resolve("", path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "xz")
}

func TestResolve_CorruptZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bogus.zip")
	require.NoError(t, os.WriteFile(path, []byte("not a zip archive"), 0o644))

	_, err := tools.Resolve("", path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zip")
}

func TestResolve_ZipMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.zip")
	f, err := os.Create(path)
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	for _, name := range []string{"a.gob", "b.gob"} {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte("data"))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())

	_, err = tools.Resolve("", path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one file")
}

func TestResolve_ZipEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.zip")
	f, err := os.Create(path)
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())

	_, err = tools.Resolve("", path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one file")
}

func TestResolve_CompoundExtensionUsesOutermost(t *testing.T) {
	raw, err := os.ReadFile(fixturePath("simple_struct.gob"))
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "data.gob.gz")
	writeGzip(t, path, raw)

	r, err := tools.Resolve("", path)
	require.NoError(t, err)
	defer r.Close()

	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, raw, got)
}

func gobEncode(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(v))
	return buf.Bytes()
}

func writeGzip(t *testing.T, path string, raw []byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	w := gzip.NewWriter(f)
	_, err = w.Write(raw)
	require.NoError(t, err)
	require.NoError(t, w.Close())
}

func writeZstd(t *testing.T, path string, raw []byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	w, err := zstd.NewWriter(f)
	require.NoError(t, err)
	_, err = w.Write(raw)
	require.NoError(t, err)
	require.NoError(t, w.Close())
}

func writeBzip2(t *testing.T, path string, raw []byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	w, err := dsnetbz2.NewWriter(f, nil)
	require.NoError(t, err)
	_, err = w.Write(raw)
	require.NoError(t, err)
	require.NoError(t, w.Close())
}

func writeXz(t *testing.T, path string, raw []byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	w, err := xz.NewWriter(f)
	require.NoError(t, err)
	_, err = w.Write(raw)
	require.NoError(t, err)
	require.NoError(t, w.Close())
}

func writeZipSingle(t *testing.T, path string, raw []byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	zw := zip.NewWriter(f)
	w, err := zw.Create("payload.gob")
	require.NoError(t, err)
	_, err = w.Write(raw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
}
