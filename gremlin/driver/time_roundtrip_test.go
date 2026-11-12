package driver_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/comparator"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

// timeTraveler exercises time.Time and *time.Time properties, which gremlin-go
// v3.8.x serialize as GraphBinary OffsetDateTime (type 0x88) instead of the
// legacy Date (0x04).
type timeTraveler struct {
	gsmtypes.Vertex
	Name     string     `json:"name"     gremlin:"name"`
	Departed time.Time  `json:"departed" gremlin:"departed"`
	Returned *time.Time `json:"returned" gremlin:"returned"`
}

func openTravelerDB(t *testing.T) *driver.GremlinDriver {
	t.Helper()
	t.Cleanup(func() { cleanDB(t) })
	db, err := driver.Open(
		DbURL, driver.Config{
			Driver: dbDriver,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}

// TestTimeRoundTripThroughOffsetDateTime pins that instants survive the
// 0x88 wire format. Comparisons use time.Equal: the driver decodes dates
// back in a fixed-offset zone, never the sender's *time.Location, so == or
// field equality against the original value would spuriously fail.
func TestTimeRoundTripThroughOffsetDateTime(t *testing.T) {
	db := openTravelerDB(t)

	cph := time.FixedZone("CET+2", 2*3600)
	departed := time.Date(2026, 3, 14, 15, 9, 26, 500, cph)
	returned := time.Date(2026, 3, 15, 8, 0, 0, 0, time.UTC)

	v := timeTraveler{Name: "roundtrip", Departed: departed, Returned: &returned}
	if err := driver.Create(db, &v); err != nil {
		t.Fatal(err)
	}

	loaded, err := driver.Model[timeTraveler](db).ID(v.GetVertexID())
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Departed.Equal(departed) {
		t.Errorf(
			"Expected departed instant %v, got %v (as %v)",
			departed, loaded.Departed, loaded.Departed.Format(time.RFC3339Nano),
		)
	}
	if loaded.Returned == nil {
		t.Fatal("Expected returned pointer to be loaded, got nil")
	}
	if !loaded.Returned.Equal(returned) {
		t.Errorf("Expected returned instant %v, got %v", returned, *loaded.Returned)
	}

	// Save must be able to overwrite date properties with new instants,
	// including through a pointer field.
	updated := time.Date(2026, 6, 1, 12, 30, 45, 0, time.UTC)
	updatedReturn := time.Date(2026, 6, 2, 12, 30, 45, 0, cph)
	v.Departed = updated
	v.Returned = &updatedReturn
	if err = driver.Save(db, &v); err != nil {
		t.Fatal(err)
	}
	reloaded, err := driver.Model[timeTraveler](db).ID(v.GetVertexID())
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.Departed.Equal(updated) {
		t.Errorf("Expected updated departed instant %v, got %v", updated, reloaded.Departed)
	}
	if reloaded.Returned == nil || !reloaded.Returned.Equal(updatedReturn) {
		t.Errorf("Expected updated returned instant %v, got %v", updatedReturn, reloaded.Returned)
	}
}

// TestOrderByDateMixedOffsets pins server-side comparability of
// OffsetDateTime values: rows written in different UTC offsets must sort
// chronologically by instant. Before the 3.8.1 orderability fix, sorting
// OffsetDateTime values failed outright.
func TestOrderByDateMixedOffsets(t *testing.T) {
	db := openTravelerDB(t)

	plus2 := time.FixedZone("PLUS2", 2*3600)
	rows := []timeTraveler{
		// Instants deliberately out of name order and zone order.
		{Name: "late", Departed: time.Date(2026, 5, 2, 12, 0, 0, 0, plus2)},  // 10:00Z
		{Name: "early", Departed: time.Date(2026, 5, 1, 10, 0, 0, 0, plus2)}, // 08:00Z
		{Name: "mid", Departed: time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)}, // 09:00Z
	}
	for i := range rows {
		if err := driver.Create(db, &rows[i]); err != nil {
			t.Fatal(err)
		}
	}

	ascending, err := driver.Model[timeTraveler](db).
		OrderBy("departed", driver.Asc).
		Find()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"early", "mid", "late"}
	if len(ascending) != len(want) {
		t.Fatalf("Expected %d rows, got %d", len(want), len(ascending))
	}
	for i, name := range want {
		if ascending[i].Name != name {
			t.Errorf("Ascending position %d: expected %s, got %s", i, name, ascending[i].Name)
		}
	}

	descending, err := driver.Model[timeTraveler](db).
		OrderBy("departed", driver.Desc).
		Find()
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"late", "mid", "early"} {
		if descending[i].Name != name {
			t.Errorf("Descending position %d: expected %s, got %s", i, name, descending[i].Name)
		}
	}
}

// TestWhereDateRangePredicate pins server-side comparison of stored
// OffsetDateTime values against query predicates.
func TestWhereDateRangePredicate(t *testing.T) {
	db := openTravelerDB(t)

	cutoff := time.Date(2026, 5, 1, 8, 30, 0, 0, time.UTC)
	rows := []timeTraveler{
		{Name: "before", Departed: time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)},
		{Name: "after", Departed: time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)},
	}
	for i := range rows {
		if err := driver.Create(db, &rows[i]); err != nil {
			t.Fatal(err)
		}
	}

	found, err := driver.Model[timeTraveler](db).
		Where("departed", comparator.GT, cutoff).
		Find()
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Name != "after" {
		names := make([]string, len(found))
		for i, f := range found {
			names[i] = f.Name
		}
		t.Errorf("Expected exactly [after] after cutoff, got %v", names)
	}
}

// TestLastModifiedStrictlyIncreasing pins that the automatic stamp survives
// the 0x88 round trip and is strictly ordered across writes.
func TestLastModifiedStrictlyIncreasing(t *testing.T) {
	db := openTravelerDB(t)

	v := timeTraveler{
		Name:     "stamper",
		Departed: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := driver.Create(db, &v); err != nil {
		t.Fatal(err)
	}

	first, err := driver.Model[timeTraveler](db).ID(v.GetVertexID())
	if err != nil {
		t.Fatal(err)
	}
	if first.LastModified.IsZero() {
		t.Fatal("Expected last_modified to be stamped on create")
	}

	err = driver.Model[timeTraveler](db).
		Where("name", comparator.EQ, "stamper").
		Update("departed", time.Date(2026, 2, 2, 2, 2, 2, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	second, err := driver.Model[timeTraveler](db).ID(v.GetVertexID())
	if err != nil {
		t.Fatal(err)
	}
	if !second.LastModified.After(first.LastModified) {
		t.Errorf(
			"Expected last_modified to increase: was %v, now %v",
			first.LastModified, second.LastModified,
		)
	}
}

// TestTakeMissingIDStaysNotFound pins that a genuinely empty result still
// maps to gsmtypes.ErrNotFound under the 3.8.1 error-surfacing change.
func TestTakeMissingIDStaysNotFound(t *testing.T) {
	db := openTravelerDB(t)

	_, err := driver.Model[timeTraveler](db).
		Where("name", comparator.EQ, "nobody").
		Take()
	if !errors.Is(err, gsmtypes.ErrNotFound) {
		t.Errorf("Expected ErrNotFound for missing row, got %v", err)
	}
}

// TestServerErrorIsNotMappedToNotFound pins the 3.8.1 Next() behavior fix:
// a server-side failure must surface as itself, not as E0903/ErrNotFound.
// Two by() modulators on valueMap are rejected by 3.8 servers (single-by is
// the rule gsm respects in ToMapTraversal), giving a deterministic failure
// that lands mid-stream.
func TestServerErrorIsNotMappedToNotFound(t *testing.T) {
	db := openTravelerDB(t)

	label := driver.GetLabel[timeTraveler]()
	_, err := db.G().V().HasLabel(label).ValueMap(true).
		By(gremlingo.T__.Identity()).
		By(gremlingo.T__.Identity()).
		Next()
	if err == nil {
		t.Fatal("Expected server error from stacked by() modulators, got nil")
	}
	if strings.Contains(err.Error(), "no results left") {
		t.Errorf("Server error must not arrive as E0903/not-found, got %v", err)
	}
	if !strings.Contains(err.Error(), "statusCode") {
		t.Errorf("Expected wrapped server error, got %v", err)
	}
}

// TestNextWithDefaultSurfacesServerError pins the nextWithDefaultValue
// hardening: when the server fails mid-iteration, the Count() path must
// report the error instead of silently returning its default.
func TestNextWithDefaultSurfacesServerError(t *testing.T) {
	db := openTravelerDB(t)

	label := driver.GetLabel[timeTraveler]()
	q := db.G().V().HasLabel(label).ValueMap(true).
		By(gremlingo.T__.Identity()).
		By(gremlingo.T__.Identity())
	_, _, err := driver.NextWithDefaultValueForTest(q, 0)
	if err == nil {
		t.Fatal("Expected nextWithDefaultValue to surface the server error, got nil")
	}
	if !strings.Contains(err.Error(), "read result set") {
		t.Errorf("Expected wrapped read-result-set error, got %v", err)
	}
}

// TestCountEmptyIsZero pins that the hardened empty path still returns the
// default for a genuinely empty result set (no error was set).
func TestCountEmptyIsZero(t *testing.T) {
	db := openTravelerDB(t)

	count, err := driver.Model[timeTraveler](db).
		Where("name", comparator.EQ, "nothing").
		Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("Expected 0 for empty match, got %d", count)
	}
}
