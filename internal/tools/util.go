package tools

import (
	"strings"

	"github.com/codepuke/gobspect"
)

// normalizeQuery strips a leading "." from the query expression (except for
// ".." which is valid recursive descent syntax). A bare "." becomes "".
func normalizeQuery(expr string) string {
	if expr == "." {
		return ""
	}
	if strings.HasPrefix(expr, ".") && !strings.HasPrefix(expr, "..") {
		return expr[1:]
	}
	return expr
}

// parseBytesFormat converts a string flag to a BytesFormat constant.
// Empty string defaults to hex.
func parseBytesFormat(s string) (gobspect.BytesFormat, bool) {
	switch strings.ToLower(s) {
	case "", "hex":
		return gobspect.BytesHex, true
	case "base64":
		return gobspect.BytesBase64, true
	case "literal":
		return gobspect.BytesLiteral, true
	default:
		return gobspect.BytesHex, false
	}
}
