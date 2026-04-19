package tools

import (
	"testing"

	"github.com/codepuke/gobspect"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeQuery(t *testing.T) {
	cases := []struct{ in, want string }{
		{".", ""},
		{".Foo", "Foo"},
		{".Foo.Bar", "Foo.Bar"},
		{"..", ".."},
		{"..Name", "..Name"},
		{"Foo", "Foo"},
		{"", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, normalizeQuery(c.in), "input: %q", c.in)
	}
}

func TestParseBytesFormat(t *testing.T) {
	for _, s := range []string{"", "hex", "HEX", "Hex"} {
		f, ok := parseBytesFormat(s)
		assert.True(t, ok, "input: %q", s)
		assert.Equal(t, gobspect.BytesHex, f, "input: %q", s)
	}
	f, ok := parseBytesFormat("base64")
	assert.True(t, ok)
	assert.Equal(t, gobspect.BytesBase64, f)

	f, ok = parseBytesFormat("BASE64")
	assert.True(t, ok)
	assert.Equal(t, gobspect.BytesBase64, f)

	f, ok = parseBytesFormat("literal")
	assert.True(t, ok)
	assert.Equal(t, gobspect.BytesLiteral, f)

	_, ok = parseBytesFormat("unknown")
	assert.False(t, ok)
}
