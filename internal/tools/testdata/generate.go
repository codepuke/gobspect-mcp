//go:build ignore

// generate.go produces gob-encoded fixture files used in tests.
//
//	go run internal/tools/testdata/generate.go
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
