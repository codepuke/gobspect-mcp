package tools

import (
	"testing"

	"github.com/codepuke/gobspect"
	"github.com/stretchr/testify/assert"
)

func TestKindOrder(t *testing.T) {
	assert.Less(t, kindOrder(gobspect.NilValue{}), kindOrder(gobspect.BoolValue{}))
	assert.Less(t, kindOrder(gobspect.BoolValue{}), kindOrder(gobspect.IntValue{}))
	assert.Equal(t, kindOrder(gobspect.IntValue{}), kindOrder(gobspect.UintValue{}))
	assert.Equal(t, kindOrder(gobspect.IntValue{}), kindOrder(gobspect.FloatValue{}))
	assert.Less(t, kindOrder(gobspect.IntValue{}), kindOrder(gobspect.StringValue{}))
	assert.Less(t, kindOrder(gobspect.StringValue{}), kindOrder(gobspect.BytesValue{}))
	assert.Less(t, kindOrder(gobspect.BytesValue{}), kindOrder(gobspect.OpaqueValue{}))
	assert.Less(t, kindOrder(gobspect.OpaqueValue{}), kindOrder(gobspect.StructValue{}))
}

func TestCompareValues_Nil(t *testing.T) {
	assert.Equal(t, 0, compareValues(gobspect.NilValue{}, gobspect.NilValue{}))
}

func TestCompareValues_CrossKind(t *testing.T) {
	assert.Equal(t, -1, compareValues(gobspect.NilValue{}, gobspect.BoolValue{}))
	assert.Equal(t, 1, compareValues(gobspect.StringValue{V: "a"}, gobspect.IntValue{V: 1}))
}

func TestCompareValues_Bool(t *testing.T) {
	assert.Equal(t, 0, compareValues(gobspect.BoolValue{V: false}, gobspect.BoolValue{V: false}))
	assert.Equal(t, -1, compareValues(gobspect.BoolValue{V: false}, gobspect.BoolValue{V: true}))
	assert.Equal(t, 1, compareValues(gobspect.BoolValue{V: true}, gobspect.BoolValue{V: false}))
}

func TestCompareValues_Int(t *testing.T) {
	assert.Equal(t, -1, compareValues(gobspect.IntValue{V: 1}, gobspect.IntValue{V: 2}))
	assert.Equal(t, 0, compareValues(gobspect.IntValue{V: 5}, gobspect.IntValue{V: 5}))
	assert.Equal(t, 1, compareValues(gobspect.IntValue{V: -1}, gobspect.IntValue{V: -2}))
}

func TestCompareValues_Uint(t *testing.T) {
	assert.Equal(t, -1, compareValues(gobspect.UintValue{V: 1}, gobspect.UintValue{V: 2}))
	assert.Equal(t, 0, compareValues(gobspect.UintValue{V: 5}, gobspect.UintValue{V: 5}))
}

func TestCompareValues_Float(t *testing.T) {
	assert.Equal(t, -1, compareValues(gobspect.FloatValue{V: 1.0}, gobspect.FloatValue{V: 2.0}))
	assert.Equal(t, 0, compareValues(gobspect.FloatValue{V: 3.14}, gobspect.FloatValue{V: 3.14}))
}

func TestCompareValues_MixedNumeric(t *testing.T) {
	// int vs uint
	assert.Equal(t, -1, compareValues(gobspect.IntValue{V: 1}, gobspect.UintValue{V: 2}))
	assert.Equal(t, 1, compareValues(gobspect.UintValue{V: 3}, gobspect.IntValue{V: 2}))
	// int vs float
	assert.Equal(t, -1, compareValues(gobspect.IntValue{V: 1}, gobspect.FloatValue{V: 1.5}))
	assert.Equal(t, -1, compareValues(gobspect.FloatValue{V: 0.5}, gobspect.IntValue{V: 1}))
	// uint vs float
	assert.Equal(t, 1, compareValues(gobspect.UintValue{V: 3}, gobspect.FloatValue{V: 2.9}))
	assert.Equal(t, -1, compareValues(gobspect.FloatValue{V: 1.0}, gobspect.UintValue{V: 2}))
}

func TestCompareValues_String(t *testing.T) {
	assert.Equal(t, -1, compareValues(gobspect.StringValue{V: "a"}, gobspect.StringValue{V: "b"}))
	assert.Equal(t, 0, compareValues(gobspect.StringValue{V: "x"}, gobspect.StringValue{V: "x"}))
	assert.Equal(t, 1, compareValues(gobspect.StringValue{V: "z"}, gobspect.StringValue{V: "a"}))
}

func TestCompareValues_Bytes(t *testing.T) {
	assert.Equal(t, -1, compareValues(gobspect.BytesValue{V: []byte{1}}, gobspect.BytesValue{V: []byte{2}}))
	assert.Equal(t, 0, compareValues(gobspect.BytesValue{V: []byte{5}}, gobspect.BytesValue{V: []byte{5}}))
}

func TestCompareValues_Opaque(t *testing.T) {
	a := gobspect.OpaqueValue{Raw: []byte{0xab}}
	b := gobspect.OpaqueValue{Raw: []byte{0xcd}}
	assert.Equal(t, -1, compareValues(a, b))
	assert.Equal(t, 0, compareValues(a, a))
	// with Decoded
	ad := gobspect.OpaqueValue{Decoded: "alpha"}
	bd := gobspect.OpaqueValue{Decoded: "beta"}
	assert.Equal(t, -1, compareValues(ad, bd))
}

func TestCompareValues_InterfaceUnwrap(t *testing.T) {
	a := gobspect.InterfaceValue{Value: gobspect.StringValue{V: "apple"}}
	b := gobspect.InterfaceValue{Value: gobspect.StringValue{V: "banana"}}
	assert.Equal(t, -1, compareValues(a, b))
}

func TestCompareValuesFold(t *testing.T) {
	assert.Equal(t, 0, compareValuesFold(gobspect.StringValue{V: "Hello"}, gobspect.StringValue{V: "hello"}))
	assert.Equal(t, -1, compareValuesFold(gobspect.StringValue{V: "apple"}, gobspect.StringValue{V: "Banana"}))
}

func TestCompareValuesFold_NonString(t *testing.T) {
	// Falls back to compareValues for non-strings.
	assert.Equal(t, -1, compareValuesFold(gobspect.IntValue{V: 1}, gobspect.IntValue{V: 2}))
}

func TestCompareValuesFold_InterfaceUnwrap(t *testing.T) {
	// Both a and b wrapped in InterfaceValue — should unwrap and fold-compare strings.
	a := gobspect.InterfaceValue{Value: gobspect.StringValue{V: "APPLE"}}
	b := gobspect.InterfaceValue{Value: gobspect.StringValue{V: "apple"}}
	assert.Equal(t, 0, compareValuesFold(a, b))
}

func TestCompareValues_Composite(t *testing.T) {
	// Two StructValues fall through to the Format-based fallback.
	a := gobspect.StructValue{TypeName: "A", Fields: []gobspect.Field{{Name: "X", Value: gobspect.IntValue{V: 1}}}}
	b := gobspect.StructValue{TypeName: "B", Fields: []gobspect.Field{{Name: "X", Value: gobspect.IntValue{V: 2}}}}
	// Result just needs to be consistent; we're testing the branch is reached.
	r := compareValues(a, b)
	assert.Contains(t, []int{-1, 0, 1}, r)
}

func TestOpaqueStr_Raw(t *testing.T) {
	v := gobspect.OpaqueValue{Raw: []byte{0xde, 0xad}}
	assert.Equal(t, "dead", opaqueStr(v))
}

func TestOpaqueStr_Decoded(t *testing.T) {
	v := gobspect.OpaqueValue{Decoded: "2024-01-01T00:00:00Z"}
	assert.Equal(t, "2024-01-01T00:00:00Z", opaqueStr(v))
}

func TestBoolInt(t *testing.T) {
	assert.Equal(t, 0, boolInt(false))
	assert.Equal(t, 1, boolInt(true))
}

func TestCmpHelpers(t *testing.T) {
	assert.Equal(t, -1, cmp64(1, 2))
	assert.Equal(t, 0, cmp64(3, 3))
	assert.Equal(t, 1, cmp64(5, 4))

	assert.Equal(t, -1, cmpU64(0, 1))
	assert.Equal(t, 0, cmpU64(7, 7))
	assert.Equal(t, 1, cmpU64(9, 8))

	assert.Equal(t, -1, cmpFloat(1.0, 2.0))
	assert.Equal(t, 0, cmpFloat(3.14, 3.14))
	assert.Equal(t, 1, cmpFloat(5.0, 4.9))
}
