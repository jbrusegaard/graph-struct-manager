package driver_test

import (
	"fmt"
	"testing"

	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
)

// TestGeneratedIDsRoundTrip pins the create-path id contract on a server with
// an assigned id manager: with a configured IDGenerator, the generator value
// is written via Property(T.Id, ...) and becomes the vertex id — even when
// the struct already carries one (createVertex deletes the struct id before
// consulting the generator). Reads must then find the vertex by that id.
// The dev server's UUID id manager converts string ids and returns them as
// UUID values, so comparisons go through canonical strings (Neptune echoes
// the string back unchanged).
func TestGeneratedIDsRoundTrip(t *testing.T) {
	t.Cleanup(func() { cleanDB(t) })

	ids := []string{
		"5f0c7a11-1111-4111-8111-111111111111",
		"5f0c7a22-2222-4222-8222-222222222222",
	}
	next := 0
	db, err := driver.Open(
		DbURL, driver.Config{
			Driver: dbDriver,
			IDGenerator: func() any {
				id := ids[next]
				if next < len(ids)-1 {
					next++
				}
				return id
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	v := testVertex{Name: "gen-first"}
	if err = driver.Create(db, &v); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(v.GetVertexID()) != ids[0] {
		t.Fatalf("Expected generated id %q on vertex, got %v", ids[0], v.GetVertexID())
	}

	// A struct-set id must not dislodge the generator's value.
	v2 := testVertex{Name: "gen-second"}
	v2.SetVertexID("ignored-explicit-id")
	if err = driver.Create(db, &v2); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(v2.GetVertexID()) != ids[1] {
		t.Fatalf(
			"Expected generator to win over the struct id %q, got %v",
			ids[1],
			v2.GetVertexID(),
		)
	}

	for i, name := range []string{"gen-first", "gen-second"} {
		loaded, loadErr := driver.Model[testVertex](db).ID(ids[i])
		if loadErr != nil {
			t.Fatalf("loading vertex %d by generated id: %v", i, loadErr)
		}
		if loaded.Name != name {
			t.Errorf("Expected to load %q by generated id, got %q", name, loaded.Name)
		}
	}

	// Bypass struct mapping: confirm the id really is the storage key.
	result, err := db.G().
		V().
		HasId(ids[1]).
		HasLabel(driver.GetLabel[testVertex]()).
		Count().
		Next()
	if err != nil {
		t.Fatal(err)
	}
	count, err := result.GetInt()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("Expected exactly 1 vertex stored under id %q, found %d", ids[1], count)
	}
}

// TestServerAssignedIDRoundTrip pins the no-generator path: the server
// assigns an id and reads must locate the vertex with it.
func TestServerAssignedIDRoundTrip(t *testing.T) {
	t.Cleanup(func() { cleanDB(t) })

	db, err := driver.Open(DbURL, driver.Config{Driver: dbDriver})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	v := testVertex{Name: "server-id"}
	if err = driver.Create(db, &v); err != nil {
		t.Fatal(err)
	}
	if v.GetVertexID() == nil {
		t.Fatal("Expected a server-assigned id after create, got nil")
	}

	loaded, err := driver.Model[testVertex](db).ID(v.GetVertexID())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "server-id" {
		t.Errorf("Expected to load server-id by its assigned id, got %q", loaded.Name)
	}
}
