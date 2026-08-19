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

const testReadLimit = 64 << 20

func TestResolve_Base64Std(t *testing.T) {
	b := gobEncode(t, 42)
	r, err := tools.Resolve(base64.StdEncoding.EncodeToString(b), "", testReadLimit)
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
	r, err := tools.Resolve(raw, "", testReadLimit)
	require.NoError(t, err)
	defer r.Close()
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, b, got)
}

func TestResolve_InvalidBase64(t *testing.T) {
	_, err := tools.Resolve("!!!not-base64!!!", "", testReadLimit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding base64")
}

func TestResolve_BothProvided(t *testing.T) {
	_, err := tools.Resolve("dGVzdA==", "somefile.gob", testReadLimit)
	require.Error(t, err)
}

func TestResolve_NeitherProvided(t *testing.T) {
	_, err := tools.Resolve("", "", testReadLimit)
	require.Error(t, err)
}

func TestResolve_File(t *testing.T) {
	r, err := tools.Resolve("", fixturePath("simple_struct.gob"), testReadLimit)
	require.NoError(t, err)
	defer r.Close()
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.NotEmpty(t, b)
}

func TestResolve_FileNotFound(t *testing.T) {
	_, err := tools.Resolve("", "/nonexistent/path/to/file.gob", testReadLimit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "opening file")
}

func TestResolve_CompressedFiles(t *testing.T) {
	raw, err := os.ReadFile(fixturePath("simple_struct.gob"))
	require.NoError(t, err)

	cases := []struct {
		name     string
		ext      string
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

			r, err := tools.Resolve("", path, testReadLimit)
			require.NoError(t, err)
			defer r.Close()

			got, err := io.ReadAll(r)
			require.NoError(t, err)
			assert.Equal(t, raw, got)
		})
	}
}

func TestResolve_UncompressedPassesThrough(t *testing.T) {
	raw, err := os.ReadFile(fixturePath("simple_struct.gob"))
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "data.weird")
	require.NoError(t, os.WriteFile(path, raw, 0o644))

	r, err := tools.Resolve("", path, testReadLimit)
	require.NoError(t, err)
	defer r.Close()

	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, raw, got)
}

// Detection is by content, so the name a compressed file happens to carry is
// irrelevant: a gzipped stream named .gob decompresses, and a plain gob stream
// named .gz does not error.
func TestResolve_ExtensionIsIgnored(t *testing.T) {
	raw, err := os.ReadFile(fixturePath("simple_struct.gob"))
	require.NoError(t, err)

	cases := []struct {
		name  string
		file  string
		write func(t *testing.T, path string, raw []byte)
	}{
		{"gzip content named .gob", "data.gob", writeGzip},
		{"gzip content named .txt", "data.txt", writeGzip},
		{"zstd content named .gob", "data.gob", writeZstd},
		{"plain gob named .gz", "data.gob.gz", writePlain},
		{"plain gob named .zip", "data.zip", writePlain},
		{"compound suffix", "data.gob.gz", writeGzip},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.file)
			tc.write(t, path, raw)

			r, err := tools.Resolve("", path, testReadLimit)
			require.NoError(t, err)
			defer r.Close()

			got, err := io.ReadAll(r)
			require.NoError(t, err)
			assert.Equal(t, raw, got)
		})
	}
}

// Compressed base64 data is decompressed too, so the data and file inputs
// accept the same bytes.
func TestResolve_CompressedBase64Data(t *testing.T) {
	raw, err := os.ReadFile(fixturePath("simple_struct.gob"))
	require.NoError(t, err)

	cases := []struct {
		name  string
		write func(t *testing.T, path string, raw []byte)
	}{
		{"gzip", writeGzip},
		{"zstd", writeZstd},
		{"bzip2", writeBzip2},
		{"xz", writeXz},
		{"zip", writeZipSingle},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "data.bin")
			tc.write(t, path, raw)
			compressed, err := os.ReadFile(path)
			require.NoError(t, err)

			r, err := tools.Resolve(base64.StdEncoding.EncodeToString(compressed), "", testReadLimit)
			require.NoError(t, err)
			defer r.Close()

			got, err := io.ReadAll(r)
			require.NoError(t, err)
			assert.Equal(t, raw, got)
		})
	}
}

// Content that claims a format by its magic bytes but is not valid in that
// format still errors — either at open or on the first read.
func TestResolve_CorruptCompressedContent(t *testing.T) {
	cases := []struct {
		name    string
		content []byte
		wantMsg string
	}{
		{"gzip", append([]byte{0x1f, 0x8b}, []byte("garbage")...), "gzip"},
		{"zstd", append([]byte{0x28, 0xb5, 0x2f, 0xfd}, []byte("garbage")...), ""},
		{"xz", append([]byte{0xfd, '7', 'z', 'X', 'Z', 0x00}, []byte("garbage")...), "xz"},
		{"zip", append([]byte("PK\x03\x04"), []byte("garbage")...), "zip"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bogus.bin")
			require.NoError(t, os.WriteFile(path, tc.content, 0o644))

			// Some codecs validate lazily, so the error may surface on read.
			r, err := tools.Resolve("", path, testReadLimit)
			if err == nil {
				_, err = io.ReadAll(r)
				r.Close()
			}
			require.Error(t, err)
			if tc.wantMsg != "" {
				assert.Contains(t, err.Error(), tc.wantMsg)
			}
		})
	}
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

	_, err = tools.Resolve("", path, testReadLimit)
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

	_, err = tools.Resolve("", path, testReadLimit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one file")
}

// The source cap bounds the compressed bytes read, which is what keeps zip's
// full-archive buffering from being unbounded.
func TestResolve_SourceCap(t *testing.T) {
	raw, err := os.ReadFile(fixturePath("simple_struct.gob"))
	require.NoError(t, err)

	t.Run("file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "data.gob")
		require.NoError(t, os.WriteFile(path, raw, 0o644))

		r, err := tools.Resolve("", path, 4)
		require.NoError(t, err)
		defer r.Close()

		_, err = io.ReadAll(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds read_limit")
	})

	t.Run("data", func(t *testing.T) {
		r, err := tools.Resolve(base64.StdEncoding.EncodeToString(raw), "", 4)
		require.NoError(t, err)
		defer r.Close()

		_, err = io.ReadAll(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds read_limit")
	})
}

func writePlain(t *testing.T, path string, raw []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, raw, 0o644))
}

func gobEncode(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(v))
	return buf.Bytes()
}

// The compressors below come in pairs: a byte-producing form the fuzz seeds
// use, and a file-writing wrapper for the Resolve tests.

func gzipToBytes(tb testing.TB, raw []byte) []byte {
	tb.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write(raw)
	require.NoError(tb, err)
	require.NoError(tb, w.Close())
	return buf.Bytes()
}

func zstdToBytes(tb testing.TB, raw []byte) []byte {
	tb.Helper()
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	require.NoError(tb, err)
	_, err = w.Write(raw)
	require.NoError(tb, err)
	require.NoError(tb, w.Close())
	return buf.Bytes()
}

func bzip2ToBytes(tb testing.TB, raw []byte) []byte {
	tb.Helper()
	var buf bytes.Buffer
	w, err := dsnetbz2.NewWriter(&buf, nil)
	require.NoError(tb, err)
	_, err = w.Write(raw)
	require.NoError(tb, err)
	require.NoError(tb, w.Close())
	return buf.Bytes()
}

func xzToBytes(tb testing.TB, raw []byte) []byte {
	tb.Helper()
	var buf bytes.Buffer
	w, err := xz.NewWriter(&buf)
	require.NoError(tb, err)
	_, err = w.Write(raw)
	require.NoError(tb, err)
	require.NoError(tb, w.Close())
	return buf.Bytes()
}

func zipToBytes(tb testing.TB, raw []byte) []byte {
	tb.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("payload.gob")
	require.NoError(tb, err)
	_, err = w.Write(raw)
	require.NoError(tb, err)
	require.NoError(tb, zw.Close())
	return buf.Bytes()
}

func writeGzip(t *testing.T, path string, raw []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, gzipToBytes(t, raw), 0o644))
}

func writeZstd(t *testing.T, path string, raw []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, zstdToBytes(t, raw), 0o644))
}

func writeBzip2(t *testing.T, path string, raw []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, bzip2ToBytes(t, raw), 0o644))
}

func writeXz(t *testing.T, path string, raw []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, xzToBytes(t, raw), 0o644))
}

func writeZipSingle(t *testing.T, path string, raw []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, zipToBytes(t, raw), 0o644))
}
