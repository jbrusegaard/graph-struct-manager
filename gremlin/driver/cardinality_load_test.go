package driver_test

import (
	"slices"
	"testing"

	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

// cardinalityMulti writes the multi-valued shape: under Neptune mode the
// driver writes set cardinality, under gremlin mode list cardinality. Both
// produce multi-valued properties on the server.
type cardinalityMulti struct {
	gsmtypes.Vertex
	Name string   `json:"name" gremlin:"name"`
	Tags []string `json:"tags" gremlin:"tags"`
}

// cardinalityScalarView reads the same stored "tags" property into a
// single-valued field, exercising the narrowing decode path.
type cardinalityScalarView struct {
	gsmtypes.Vertex
	Name string `json:"name" gremlin:"name"`
	Tags string `json:"tags" gremlin:"tags"`
}

// TestSingleValuedMultiPropertyUnwraps pins the current valueMap By-modulator
// contract: a set/list-cardinality property holding exactly ONE value is
// unfolded to a bare scalar before unload, so no narrowing is reported.
//
// The elementMap read-path migration changes this shape: elementMap keeps
// set/list properties list-wrapped, so the same load becomes a one-element
// list narrowed to the scalar with 0 discarded. That phase must update this
// test's assertions deliberately — a silent failure here means the
// migration shipped an unnoticed load-shape change.
func TestSingleValuedMultiPropertyUnwraps(t *testing.T) {
	db := openTravelerDB(t)

	v := cardinalityMulti{Name: "solo", Tags: []string{"solo-tag"}}
	if err := driver.Create(db, &v); err != nil {
		t.Fatal(err)
	}

	label := driver.GetLabel[cardinalityMulti]()
	q := driver.ToMapTraversal(db.G().V().HasLabel(label).Has("name", "solo"), nil)
	result, err := q.Next()
	if err != nil {
		t.Fatal(err)
	}

	var view cardinalityScalarView
	narrowed, err := driver.UnloadGremlinResultIntoStructReportingNarrowed(&view, result)
	if err != nil {
		t.Fatal(err)
	}
	if view.Tags != "solo-tag" {
		t.Errorf("Expected scalar field to load %q, got %q", "solo-tag", view.Tags)
	}
	if len(narrowed) != 0 {
		t.Errorf("Expected no narrowing report for unfolded single value, got %+v", narrowed)
	}
}

// TestMultiValuedPropertyNarrowsIntoScalar pins that a multi-valued property
// loaded into a single-valued field keeps the first value and reports the
// discarded remainder.
func TestMultiValuedPropertyNarrowsIntoScalar(t *testing.T) {
	db := openTravelerDB(t)

	v := cardinalityMulti{Name: "pair", Tags: []string{"alpha", "beta"}}
	if err := driver.Create(db, &v); err != nil {
		t.Fatal(err)
	}

	label := driver.GetLabel[cardinalityMulti]()
	q := driver.ToMapTraversal(db.G().V().HasLabel(label).Has("name", "pair"), nil)
	result, err := q.Next()
	if err != nil {
		t.Fatal(err)
	}

	var view cardinalityScalarView
	narrowed, err := driver.UnloadGremlinResultIntoStructReportingNarrowed(&view, result)
	if err != nil {
		t.Fatal(err)
	}
	// Set cardinality may reorder values; the kept value must be one of the
	// two written, and exactly one must be reported discarded.
	if view.Tags != "alpha" && view.Tags != "beta" {
		t.Errorf("Expected kept value to be one of the written tags, got %q", view.Tags)
	}
	if len(narrowed) != 1 {
		t.Fatalf("Expected one narrowed property report, got %+v", narrowed)
	}
	if narrowed[0].Property != "tags" || narrowed[0].Discarded != 1 {
		t.Errorf(
			"Expected tags/1-discarded narrowing, got property=%q discarded=%d",
			narrowed[0].Property, narrowed[0].Discarded,
		)
	}
}

// TestSliceFieldKeepsAllValues pins the same-cardinality load: a slice field
// must receive every stored value.
func TestSliceFieldKeepsAllValues(t *testing.T) {
	db := openTravelerDB(t)

	v := cardinalityMulti{Name: "keepall", Tags: []string{"alpha", "beta"}}
	if err := driver.Create(db, &v); err != nil {
		t.Fatal(err)
	}

	loaded, err := driver.Model[cardinalityMulti](db).ID(v.GetVertexID())
	if err != nil {
		t.Fatal(err)
	}
	got := slices.Clone(loaded.Tags)
	slices.Sort(got)
	if !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Errorf("Expected both tags to load, got %v", loaded.Tags)
	}
}
