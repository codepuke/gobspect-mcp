package tools

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/codepuke/gobspect"
)

// kindOrder returns the bucket index for cross-kind comparison.
// Order: NilValue < BoolValue < numeric < StringValue < BytesValue < OpaqueValue < composite.
func kindOrder(v gobspect.Value) int {
	switch v.(type) {
	case gobspect.NilValue:
		return 0
	case gobspect.BoolValue:
		return 1
	case gobspect.IntValue, gobspect.UintValue, gobspect.FloatValue:
		return 2
	case gobspect.StringValue:
		return 3
	case gobspect.BytesValue:
		return 4
	case gobspect.OpaqueValue:
		return 5
	default:
		return 6
	}
}

// compareValues returns -1, 0, or +1 ordering a before, equal to, or after b.
func compareValues(a, b gobspect.Value) int {
	if iv, ok := a.(gobspect.InterfaceValue); ok {
		a = iv.Value
	}
	if iv, ok := b.(gobspect.InterfaceValue); ok {
		b = iv.Value
	}

	oa, ob := kindOrder(a), kindOrder(b)
	if oa != ob {
		return cmpInt(oa, ob)
	}

	switch av := a.(type) {
	case gobspect.NilValue:
		return 0

	case gobspect.BoolValue:
		bv := b.(gobspect.BoolValue)
		return cmpInt(boolInt(av.V), boolInt(bv.V))

	case gobspect.IntValue:
		switch bv := b.(type) {
		case gobspect.IntValue:
			return cmp64(av.V, bv.V)
		case gobspect.UintValue:
			return cmpFloat(float64(av.V), float64(bv.V))
		case gobspect.FloatValue:
			return cmpFloat(float64(av.V), bv.V)
		}

	case gobspect.UintValue:
		switch bv := b.(type) {
		case gobspect.UintValue:
			return cmpU64(av.V, bv.V)
		case gobspect.IntValue:
			return cmpFloat(float64(av.V), float64(bv.V))
		case gobspect.FloatValue:
			return cmpFloat(float64(av.V), bv.V)
		}

	case gobspect.FloatValue:
		switch bv := b.(type) {
		case gobspect.FloatValue:
			return cmpFloat(av.V, bv.V)
		case gobspect.IntValue:
			return cmpFloat(av.V, float64(bv.V))
		case gobspect.UintValue:
			return cmpFloat(av.V, float64(bv.V))
		}

	case gobspect.StringValue:
		bv := b.(gobspect.StringValue)
		return cmpInt(strings.Compare(av.V, bv.V), 0)

	case gobspect.BytesValue:
		bv := b.(gobspect.BytesValue)
		return cmpInt(bytes.Compare(av.V, bv.V), 0)

	case gobspect.OpaqueValue:
		bv := b.(gobspect.OpaqueValue)
		return cmpInt(strings.Compare(opaqueStr(av), opaqueStr(bv)), 0)
	}

	return cmpInt(strings.Compare(gobspect.Format(a), gobspect.Format(b)), 0)
}

// compareValuesFold is like compareValues but string comparisons use ToLower.
func compareValuesFold(a, b gobspect.Value) int {
	if iv, ok := a.(gobspect.InterfaceValue); ok {
		a = iv.Value
	}
	if iv, ok := b.(gobspect.InterfaceValue); ok {
		b = iv.Value
	}
	if av, ok := a.(gobspect.StringValue); ok {
		if bv, ok := b.(gobspect.StringValue); ok {
			return cmpInt(strings.Compare(strings.ToLower(av.V), strings.ToLower(bv.V)), 0)
		}
	}
	return compareValues(a, b)
}

func opaqueStr(v gobspect.OpaqueValue) string {
	if v.Decoded != nil {
		return fmt.Sprint(v.Decoded)
	}
	return hex.EncodeToString(v.Raw)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func cmp64(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func cmpU64(a, b uint64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func cmpFloat(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
