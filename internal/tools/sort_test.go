package tools

import (
	"iter"
	"testing"

	"github.com/codepuke/gobspect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSortSpec_EmptyKey(t *testing.T) {
	_, err := ParseSortSpec("", false, false, false)
	require.Error(t, err)
}

func TestParseSortSpec_EmptyFieldName(t *testing.T) {
	_, err := ParseSortSpec("a,,b", false, false, false)
	require.Error(t, err)
}

func TestParseSortSpec_MultiKey(t *testing.T) {
	spec, err := ParseSortSpec(" Name , ID ", false, false, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"Name", "ID"}, spec.Keys)
}

func TestSortMatches_DropMissing(t *testing.T) {
	withKey := gobspect.StructValue{Fields: []gobspect.Field{
		{Name: "Score", Value: gobspect.IntValue{V: 10}},
	}}
	withoutKey := gobspect.StructValue{Fields: []gobspect.Field{
		{Name: "Other", Value: gobspect.StringValue{V: "x"}},
	}}
	spec := SortSpec{Keys: []string{"Score"}, DropMissing: true}

	results := sortMatches(seqValues(withKey, withoutKey), spec)
	require.Len(t, results, 1)
	assert.Equal(t, withKey, results[0])
}

func TestSortMatches_DropMissing_AnyKeyMatches(t *testing.T) {
	row := gobspect.StructValue{Fields: []gobspect.Field{
		{Name: "B", Value: gobspect.IntValue{V: 1}},
	}}
	spec := SortSpec{Keys: []string{"A", "B"}, DropMissing: true}

	results := sortMatches(seqValues(row), spec)
	require.Len(t, results, 1)
}

func TestExtractSortKey_InterfaceValue(t *testing.T) {
	inner := gobspect.StructValue{Fields: []gobspect.Field{
		{Name: "X", Value: gobspect.IntValue{V: 42}},
	}}
	wrapped := gobspect.InterfaceValue{Value: inner}

	v, ok := extractSortKey(wrapped, "X")
	require.True(t, ok)
	assert.Equal(t, gobspect.IntValue{V: 42}, v)
}

func TestExtractSortKey_NonStruct(t *testing.T) {
	_, ok := extractSortKey(gobspect.StringValue{V: "hello"}, "field")
	assert.False(t, ok)
}

func TestSortSpec_Compare_Desc(t *testing.T) {
	a := gobspect.StructValue{Fields: []gobspect.Field{{Name: "N", Value: gobspect.IntValue{V: 1}}}}
	b := gobspect.StructValue{Fields: []gobspect.Field{{Name: "N", Value: gobspect.IntValue{V: 2}}}}
	spec := SortSpec{Keys: []string{"N"}, Desc: true}
	assert.Equal(t, 1, spec.Compare(a, b)) // desc: 1 > 2 reversed
}

func TestSortSpec_Compare_Fold(t *testing.T) {
	a := gobspect.StructValue{Fields: []gobspect.Field{{Name: "S", Value: gobspect.StringValue{V: "Apple"}}}}
	b := gobspect.StructValue{Fields: []gobspect.Field{{Name: "S", Value: gobspect.StringValue{V: "apple"}}}}
	spec := SortSpec{Keys: []string{"S"}, Fold: true}
	assert.Equal(t, 0, spec.Compare(a, b))
}

func TestSeqOf_EarlyExit(t *testing.T) {
	vals := []gobspect.Value{
		gobspect.IntValue{V: 1},
		gobspect.IntValue{V: 2},
		gobspect.IntValue{V: 3},
	}
	seq := seqOf(vals)
	var got []gobspect.Value
	seq(func(v gobspect.Value) bool {
		got = append(got, v)
		return len(got) < 2 // stop after 2
	})
	assert.Len(t, got, 2)
}

// seqValues wraps values as an iter.Seq.
func seqValues(vals ...gobspect.Value) iter.Seq[gobspect.Value] {
	return func(yield func(gobspect.Value) bool) {
		for _, v := range vals {
			if !yield(v) {
				return
			}
		}
	}
}
