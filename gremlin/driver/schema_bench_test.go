package driver

import (
	"testing"

	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

type benchVertex struct {
	gsmtypes.Vertex
	Name   string   `gremlin:"name"`
	Email  string   `gremlin:"email,omitempty"`
	Age    int      `gremlin:"age"`
	Tags   []string `gremlin:"tags"`
	Score  float64  `gremlin:"score"`
	Active bool     `gremlin:"active"`
}

func BenchmarkStructToMap(b *testing.B) {
	v := &benchVertex{Name: "a", Email: "b", Age: 1, Tags: []string{"x"}, Score: 1.5, Active: true}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := structToMap(v); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnloadMap(b *testing.B) {
	mapResult := map[any]any{
		"name":   "a",
		"email":  "b",
		"age":    1,
		"tags":   []any{"x", "y"},
		"score":  1.5,
		"active": true,
	}
	b.ReportAllocs()
	for b.Loop() {
		var v benchVertex
		if err := unloadGremlinMapIntoStruct(&v, mapResult); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewQuery(b *testing.B) {
	db := &GremlinDriver{}
	b.ReportAllocs()
	for b.Loop() {
		_ = NewQuery[benchVertex](db)
	}
}
