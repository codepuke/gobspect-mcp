//go:build ignore

// generate.go produces gob-encoded fixture files used in tests.
//
//	cd internal/tools && go run testdata/generate.go
//
// Note that map_value.gob encodes a map, whose gob byte order is not stable
// across runs; regenerating it produces spurious diffs.
package main

import (
	"bytes"
	"encoding/gob"
	"os"
)

type SimpleStruct struct {
	ID   int
	Name string
}

type NestedStruct struct {
	Inner SimpleStruct
	Score float64
}

// PersonA and PersonB share a Name field so a sort key resolves in both
// partitions, which is what makes global vs. per-partition sorting
// distinguishable in hetero=partition mode.
type PersonA struct {
	Name string
	Age  int
}

type PersonB struct {
	Name string
	City string
}

type Animal interface{}
type Dog struct{ Breed string }
type Cat struct{ Indoor bool }

type AnimalHolder struct{ Pet Animal }

func init() {
	gob.Register(Dog{})
	gob.Register(Cat{})
}

func main() {
	write("testdata/simple_struct.gob", SimpleStruct{ID: 1, Name: "alice"})

	writeMulti("testdata/multi_value.gob",
		SimpleStruct{ID: 3, Name: "charlie"},
		SimpleStruct{ID: 1, Name: "alice"},
		SimpleStruct{ID: 2, Name: "bob"},
	)

	write("testdata/nested.gob", NestedStruct{
		Inner: SimpleStruct{ID: 7, Name: "inner"},
		Score: 9.5,
	})

	write("testdata/map_value.gob", map[string]int{"x": 1, "y": 2})

	write("testdata/slice_value.gob", []string{"a", "b", "c"})

	writeMulti("testdata/hetero.gob",
		SimpleStruct{ID: 1, Name: "first"},
		NestedStruct{Inner: SimpleStruct{ID: 2, Name: "second"}, Score: 1.0},
	)

	// Interleaved types whose names sort differently within a partition than
	// they do globally.
	writeMulti("testdata/hetero_sort.gob",
		PersonA{Name: "carol", Age: 30},
		PersonB{Name: "zoe", City: "oslo"},
		PersonA{Name: "alice", Age: 41},
		PersonB{Name: "amy", City: "kyoto"},
	)
}

func write(path string, vals ...any) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, v := range vals {
		if err := enc.Encode(v); err != nil {
			panic(path + ": " + err.Error())
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		panic(err)
	}
}

func writeMulti(path string, vals ...any) { write(path, vals...) }
