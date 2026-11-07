package driver_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

// These tests pin the README's "Custom IDs" promise: an ID set on the struct
// before Create/CreateEdge is honoured instead of being replaced. Custom IDs
// are UUIDs because the checked-in TinkerGraph config runs the AUTO (UUID)
// id manager, which rejects other id types server-side. The server returns
// ids as java.util.UUID, which gremlin-go decodes to uuid.UUID, so structs
// declared with uuid.UUID ids keep a value that compares equal afterwards.

type customIDVertex struct {
	gsmtypes.Vertex
	Name string `gremlin:"name"`
}

type likesThing struct {
	gsmtypes.Edge
	Rating int `gremlin:"rating"`
}

func TestCreateHonoursCustomID(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(cleanDB)

	customID := uuid.New()
	v := customIDVertex{
		Vertex: gsmtypes.Vertex{ID: customID},
		Name:   "custom",
	}
	if err := driver.Create(db, &v); err != nil {
		t.Fatal(err)
	}
	if v.ID != customID {
		t.Fatalf("expected struct to keep ID %v, got %v (%T)", customID, v.ID, v.ID)
	}

	loaded, err := driver.Model[customIDVertex](db).ID(customID)
	if err != nil {
		t.Fatalf("vertex not found under custom ID: %v", err)
	}
	if loaded.Name != "custom" {
		t.Errorf("expected name %q, got %q", "custom", loaded.Name)
	}

	// id must be the element ID only, never also a regular property.
	result, err := db.G().V(customID).ValueMap().Next()
	if err != nil {
		t.Fatal(err)
	}
	props, ok := result.GetInterface().(map[any]any)
	if !ok {
		t.Fatalf("unexpected ValueMap result type %T", result.GetInterface())
	}
	if _, found := props["id"]; found {
		t.Error("id should not be written as a regular property")
	}
}

func TestCreateEdgeHonoursCustomID(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(cleanDB)
	person, topic := seedEdgeEndpoints(t, db)

	customID := uuid.New()
	sub := likesThing{
		Edge:   gsmtypes.Edge{ID: customID},
		Rating: 5,
	}
	if err := driver.CreateEdge(db, &sub, &person, &topic); err != nil {
		t.Fatal(err)
	}
	if sub.ID != customID {
		t.Fatalf("expected struct to keep edge ID %v, got %v (%T)", customID, sub.ID, sub.ID)
	}

	loaded, err := driver.Edge[likesThing](db).ID(customID)
	if err != nil {
		t.Fatalf("edge not found under custom ID: %v", err)
	}
	if loaded.Rating != 5 {
		t.Errorf("expected rating 5, got %v", loaded.Rating)
	}
}

func TestCreateCustomIDWinsOverIDGenerator(t *testing.T) {
	db, err := driver.Open(
		DbURL, driver.Config{
			Driver: dbDriver,
			IDGenerator: func() any {
				return uuid.New()
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	t.Cleanup(cleanDB)

	explicitID := uuid.New()
	set := customIDVertex{
		Vertex: gsmtypes.Vertex{ID: explicitID},
		Name:   "set",
	}
	if err = driver.Create(db, &set); err != nil {
		t.Fatal(err)
	}
	if set.ID != explicitID {
		t.Errorf("struct ID should win over IDGenerator, got %v", set.ID)
	}
	if _, err = driver.Model[customIDVertex](db).ID(explicitID); err != nil {
		t.Errorf("vertex not found under explicit ID: %v", err)
	}

	// With no ID on the struct the IDGenerator still supplies one.
	unset := customIDVertex{Name: "unset"}
	if err = driver.Create(db, &unset); err != nil {
		t.Fatal(err)
	}
	if unset.ID == nil {
		t.Fatal("expected IDGenerator to supply an ID when the struct has none")
	}
	if unset.ID == explicitID {
		t.Error("generator produced the same ID as the explicit one")
	}
	if _, err = driver.Model[customIDVertex](db).ID(unset.ID); err != nil {
		t.Errorf("vertex not found under generated ID %v: %v", unset.ID, err)
	}
}
