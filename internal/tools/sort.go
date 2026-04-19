package tools

import (
	"fmt"
	"iter"
	"sort"
	"strings"

	"github.com/codepuke/gobspect"
)

// SortSpec describes how to sort rows from a gob stream.
type SortSpec struct {
	Keys        []string
	Desc        bool
	Fold        bool
	DropMissing bool
}

// ParseSortSpec parses comma-separated field names and sort modifiers.
func ParseSortSpec(keysFlag string, desc, fold, dropMissing bool) (SortSpec, error) {
	if keysFlag == "" {
		return SortSpec{}, fmt.Errorf("sort keys must not be empty")
	}
	parts := strings.Split(keysFlag, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		k := strings.TrimSpace(p)
		if k == "" {
			return SortSpec{}, fmt.Errorf("sort keys must not contain empty field names")
		}
		keys = append(keys, k)
	}
	return SortSpec{Keys: keys, Desc: desc, Fold: fold, DropMissing: dropMissing}, nil
}

// Compare returns -1, 0, or +1 for a vs b by the spec's keys.
func (s SortSpec) Compare(a, b gobspect.Value) int {
	cmpFn := compareValues
	if s.Fold {
		cmpFn = compareValuesFold
	}
	for _, key := range s.Keys {
		av, _ := extractSortKey(a, key)
		bv, _ := extractSortKey(b, key)
		r := cmpFn(av, bv)
		if r != 0 {
			if s.Desc {
				return -r
			}
			return r
		}
	}
	return 0
}

// sortMatches collects all values from matches, optionally filters rows missing
// all sort keys, then sorts stably.
func sortMatches(matches iter.Seq[gobspect.Value], spec SortSpec) []gobspect.Value {
	var buf []gobspect.Value
	for v := range matches {
		buf = append(buf, v)
	}
	if spec.DropMissing {
		kept := buf[:0]
		for _, row := range buf {
			for _, key := range spec.Keys {
				if _, ok := extractSortKey(row, key); ok {
					kept = append(kept, row)
					break
				}
			}
		}
		buf = kept
	}
	sort.SliceStable(buf, func(i, j int) bool {
		return spec.Compare(buf[i], buf[j]) < 0
	})
	return buf
}

// seqOf converts a slice to an iter.Seq.
func seqOf(vals []gobspect.Value) iter.Seq[gobspect.Value] {
	return func(yield func(gobspect.Value) bool) {
		for _, v := range vals {
			if !yield(v) {
				return
			}
		}
	}
}

// extractSortKey returns the named field value from v (unwrapping InterfaceValue).
func extractSortKey(v gobspect.Value, field string) (gobspect.Value, bool) {
	if iv, ok := v.(gobspect.InterfaceValue); ok {
		v = iv.Value
	}
	sv, ok := v.(gobspect.StructValue)
	if !ok {
		return gobspect.NilValue{}, false
	}
	for _, f := range sv.Fields {
		if f.Name == field {
			return f.Value, true
		}
	}
	return gobspect.NilValue{}, false
}
