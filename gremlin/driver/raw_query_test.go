package driver_test

import (
	"strings"
	"testing"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
)

// TestRawQueryRejectsTraversalValues pins the guard against the 3.8-removed
// has(key, traversal) form: RawQuery.Has must fail fast with guidance to use
// Where, instead of letting the server reject the bytecode mid-flight.
func TestRawQueryRejectsTraversalValues(t *testing.T) {
	db := openTravelerDB(t)

	label := driver.GetLabel[timeTraveler]()
	_, err := db.Label(label).
		Has("name", gremlingo.T__.Has("name", "roundtrip")).
		ToList()
	if err == nil {
		t.Fatal("Expected traversal-valued Has to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "use Where instead") {
		t.Errorf("Expected guard guidance in error, got %v", err)
	}
}

// TestRawQueryNextReturnsElementMap smoke-tests the elementMap-backed Next:
// id, label and the filtered property must all be present in the map.
func TestRawQueryNextReturnsElementMap(t *testing.T) {
	db := openTravelerDB(t)

	label := driver.GetLabel[timeTraveler]()
	seed := timeTraveler{
		Name:     "raw-ok",
		Departed: time.Date(2026, 4, 4, 4, 4, 4, 0, time.UTC),
	}
	if err := driver.Create(db, &seed); err != nil {
		t.Fatal(err)
	}

	result, err := db.Label(label).Has("name", "raw-ok").Next()
	if err != nil {
		t.Fatal(err)
	}
	row, ok := result.GetInterface().(map[any]any)
	if !ok {
		t.Fatalf("Expected elementMap map from Next, got %T", result.GetInterface())
	}
	if row["name"] != "raw-ok" {
		t.Errorf("Expected name in elementMap result, got %v", row["name"])
	}
	if row["id"] == nil {
		t.Error("Expected id key in elementMap result")
	}
	if row["label"] != label {
		t.Errorf("Expected label %q in result, got %v", label, row["label"])
	}
}
