package tools

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Resource limits. Every tool input is untrusted: the server decompresses
// caller-supplied bytes and returns the whole formatted result as a single MCP
// text response, so both the decoded byte count and the response size need a
// ceiling. Callers may raise either limit up to the hard maximum; zero is not
// accepted, because "unlimited" is exactly the configuration these guard
// against.
const (
	defaultReadLimit int64 = 64 << 20 // 64 MiB of decompressed input
	maxReadLimit     int64 = 1 << 30  // 1 GiB

	defaultOutputLimit = 1 << 20  // 1 MiB of response text
	maxOutputLimit     = 16 << 20 // 16 MiB
)

// limitedSource caps the bytes read from a compressed (or raw) input source.
// The read limit proper applies to the decompressed side via
// gobspect.WithReadLimit; this cap bounds the source itself, which matters
// because decompress.Reader buffers zip archives fully in memory. It reports an
// error rather than truncating, so an over-long input never looks like a
// corrupt gob stream.
type limitedSource struct {
	r     io.Reader
	limit int64
	read  int64
}

func (l *limitedSource) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	l.read += int64(n)
	if l.read > l.limit {
		return n, fmt.Errorf("input exceeds read_limit of %d bytes", l.limit)
	}
	return n, err
}

// errOutputLimit stops a pipeline once the response buffer is full. It travels
// back through gq.Pipeline wrapped in a *gq.SinkError, which supports
// errors.Is, and means "stop cleanly" rather than "fail".
var errOutputLimit = errors.New("output limit reached")

// resolveLimits validates the caller-supplied limits and fills in defaults.
// A nil pointer means "unset"; an explicit zero is rejected, because unlimited
// is the configuration these limits exist to prevent.
func resolveLimits(readLimit *int64, outputLimit *int) (int64, int, error) {
	read := defaultReadLimit
	if readLimit != nil {
		read = *readLimit
		switch {
		case read <= 0:
			return 0, 0, fmt.Errorf("read_limit must be positive; unlimited is not supported")
		case read > maxReadLimit:
			return 0, 0, fmt.Errorf("read_limit %d exceeds the maximum of %d", read, maxReadLimit)
		}
	}

	output := defaultOutputLimit
	if outputLimit != nil {
		output = *outputLimit
		switch {
		case output <= 0:
			return 0, 0, fmt.Errorf("output_limit must be positive; unlimited is not supported")
		case output > maxOutputLimit:
			return 0, 0, fmt.Errorf("output_limit %d exceeds the maximum of %d", output, maxOutputLimit)
		}
	}

	return read, output, nil
}

// truncationNotice is appended, on its own line, to any response cut short by
// the output limit.
func truncationNotice(limit int) string {
	return fmt.Sprintf("... output truncated at %d bytes; narrow the query, lower limit, or raise output_limit\n", limit)
}

// capWriter passes bytes through until limit is reached, then reports
// errOutputLimit. It is used for output whose record boundaries the handler
// cannot see, such as the tabular printer's buffered rows.
type capWriter struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

// Write reports len(p) even when it stores less, so that intermediate writers
// report errOutputLimit rather than masking it as io.ErrShortWrite — the
// sentinel has to survive for errors.Is to recognize the stop.
func (c *capWriter) Write(p []byte) (int, error) {
	if room := c.limit - c.buf.Len(); room < len(p) {
		c.truncated = true
		if room > 0 {
			c.buf.Write(p[:room])
		}
		return len(p), errOutputLimit
	}
	return c.buf.Write(p)
}

// clampToLine trims s to the last complete line that fits within limit, so a
// truncated response never ends mid-record.
func clampToLine(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	s = s[:limit]
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[:i+1]
	}
	return ""
}

// capText applies the output limit to a fully-formed text response, truncating
// at a line boundary and appending the notice.
func capText(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return clampToLine(s, limit) + truncationNotice(limit)
}
