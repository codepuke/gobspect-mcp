package tools_test

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"os"
	"testing"
)

// gobBase64 encodes values into a gob stream and returns the base64 string.
func gobBase64(t *testing.T, vals ...any) string {
	t.Helper()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, v := range vals {
		if err := enc.Encode(v); err != nil {
			t.Fatalf("gobBase64: encode: %v", err)
		}
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// fixtureBase64 reads a testdata file and returns it as a base64 string.
func fixtureBase64(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("fixtureBase64: read %s: %v", name, err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// fixturePath returns the absolute path to a testdata file.
func fixturePath(name string) string {
	return "testdata/" + name
}
