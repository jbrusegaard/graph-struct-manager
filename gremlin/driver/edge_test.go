package driver_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jbrusegaard/graph-struct-manager/comparator"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

type edgeTestTopic struct {
	gsmtypes.Vertex
	Title string `gremlin:"title"`
}

type edgeTestPerson struct {
	gsmtypes.Vertex
	Name   string          `gremlin:"name"`
	Topics []edgeTestTopic `gremlinEdge:"subscribes_to"`
}

// subscribesTo resolves to the edge label "subscribes_to" via snake casing.
type subscribesTo struct {
	gsmtypes.Edge
	Weight float64 `gremlin:"weight"`
	Notes  string  `gremlin:"notes,omitempty"`
}

func openEdgeTestDB(t *testing.T) *driver.GremlinDriver {
	t.Helper()
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

func seedEdgeEndpoints(
	t *testing.T, db *driver.GremlinDriver,
) (edgeTestPerson, edgeTestTopic) {
	t.Helper()
	person := edgeTestPerson{Name: "alice"}
	if err := driver.Create(db, &person); err != nil {
		t.Fatal(err)
	}
	topic := edgeTestTopic{Title: "graphs"}
	if err := driver.Create(db, &topic); err != nil {
		t.Fatal(err)
	}
	return person, topic
}

func TestCreateEdge(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(cleanDB)
	person, topic := seedEdgeEndpoints(t, db)

	sub := subscribesTo{Weight: 1.5, Notes: "daily"}
	if err := driver.CreateEdge(db, &sub, &person, &topic); err != nil {
		t.Fatal(err)
	}
	if sub.ID == nil {
		t.Error("edge ID should be set after create")
	}
	if sub.CreatedAt.IsZero() {
		t.Error("edge CreatedAt should be stamped on create")
	}
	if sub.LastModified.IsZero() {
		t.Error("edge LastModified should be stamped on create")
	}

	loaded, err := driver.Edge[subscribesTo](db).ID(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Weight != sub.Weight {
		t.Errorf("expected weight %v, got %v", sub.Weight, loaded.Weight)
	}
	if loaded.Notes != sub.Notes {
		t.Errorf("expected notes %q, got %q", sub.Notes, loaded.Notes)
	}
	if loaded.CreatedAt.IsZero() || loaded.LastModified.IsZero() {
		t.Error("loaded edge timestamps should not be zero")
	}

	outV, err := db.G().E(sub.ID).OutV().Id().Next()
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(outV.GetInterface()) != fmt.Sprint(person.ID) {
		t.Errorf("expected out vertex %v, got %v", person.ID, outV.GetInterface())
	}
	inV, err := db.G().E(sub.ID).InV().Id().Next()
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(inV.GetInterface()) != fmt.Sprint(topic.ID) {
		t.Errorf("expected in vertex %v, got %v", topic.ID, inV.GetInterface())
	}
}

func TestCreateEdgeWithRawIDs(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(cleanDB)
	person, topic := seedEdgeEndpoints(t, db)

	sub := subscribesTo{Weight: 2}
	if err := driver.CreateEdge(db, &sub, person.ID, topic.ID); err != nil {
		t.Fatal(err)
	}
	if sub.ID == nil {
		t.Error("edge ID should be set after create")
	}
	loaded, err := driver.Edge[subscribesTo](db).ID(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Weight != sub.Weight {
		t.Errorf("expected weight %v, got %v", sub.Weight, loaded.Weight)
	}
}

func TestCreateEdgeErrors(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(cleanDB)
	person, topic := seedEdgeEndpoints(t, db)

	t.Run(
		"NilEndpoint", func(t *testing.T) {
			sub := subscribesTo{}
			err := driver.CreateEdge(db, &sub, nil, &topic)
			if err == nil || !strings.Contains(err.Error(), "create edge from vertex") {
				t.Errorf("expected nil endpoint error, got %v", err)
			}
		},
	)
	t.Run(
		"EndpointWithoutID", func(t *testing.T) {
			sub := subscribesTo{}
			unsaved := edgeTestPerson{Name: "bob"}
			err := driver.CreateEdge(db, &sub, &unsaved, &topic)
			if err == nil || !strings.Contains(err.Error(), "vertex has no id") {
				t.Errorf("expected missing id error, got %v", err)
			}
		},
	)
	t.Run(
		"FromVertexNotFound", func(t *testing.T) {
			sub := subscribesTo{}
			missingID := uuid.New().String()
			err := driver.CreateEdge(db, &sub, missingID, &topic)
			if err == nil || !strings.Contains(err.Error(), "not found") {
				t.Errorf("expected not found error, got %v", err)
			}
		},
	)
	t.Run(
		"NotAnEdgeStruct", func(t *testing.T) {
			notAnEdge := edgeTestTopic{Title: "not an edge"}
			err := driver.CreateEdge(db, &notAnEdge, &person, &topic)
			if err == nil || !strings.Contains(err.Error(), "EdgeType") {
				t.Errorf("expected EdgeType error, got %v", err)
			}
		},
	)
}

func TestSaveEdge(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(cleanDB)
	person, topic := seedEdgeEndpoints(t, db)

	sub := subscribesTo{Weight: 1}
	// Save with no ID creates the edge.
	if err := driver.SaveEdge(db, &sub, &person, &topic); err != nil {
		t.Fatal(err)
	}
	if sub.ID == nil {
		t.Fatal("edge ID should be set after save-create")
	}

	sub.Weight = 3.25
	sub.Notes = "updated"
	// Save with an ID updates the existing edge in place.
	if err := driver.SaveEdge(db, &sub, &person, &topic); err != nil {
		t.Fatal(err)
	}

	count, err := driver.Edge[subscribesTo](db).Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 edge after save-update, got %d", count)
	}
	loaded, err := driver.Edge[subscribesTo](db).ID(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Weight != 3.25 {
		t.Errorf("expected weight 3.25, got %v", loaded.Weight)
	}
	if loaded.Notes != "updated" {
		t.Errorf("expected notes %q, got %q", "updated", loaded.Notes)
	}
	if loaded.LastModified.Before(loaded.CreatedAt) {
		t.Error("LastModified should not be before CreatedAt after update")
	}

	t.Run(
		"NotAnEdgeStruct", func(t *testing.T) {
			notAnEdge := edgeTestTopic{Title: "not an edge"}
			err := driver.SaveEdge(db, &notAnEdge, &person, &topic)
			if err == nil || !strings.Contains(err.Error(), "EdgeType") {
				t.Errorf("expected EdgeType error, got %v", err)
			}
		},
	)
}

func TestEdgeQueryBuilder(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(cleanDB)
	person, _ := seedEdgeEndpoints(t, db)

	weights := []float64{1, 2, 3}
	for _, weight := range weights {
		topic := edgeTestTopic{Title: fmt.Sprintf("topic-%v", weight)}
		if err := driver.Create(db, &topic); err != nil {
			t.Fatal(err)
		}
		sub := subscribesTo{Weight: weight}
		if err := driver.CreateEdge(db, &sub, &person, &topic); err != nil {
			t.Fatal(err)
		}
	}

	t.Run(
		"FindWhere", func(t *testing.T) {
			results, err := driver.Edge[subscribesTo](db).
				Where("weight", comparator.GT, 1.0).
				Find()
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 2 {
				t.Errorf("expected 2 edges, got %d", len(results))
			}
		},
	)
	t.Run(
		"Count", func(t *testing.T) {
			count, err := driver.Edge[subscribesTo](db).Count()
			if err != nil {
				t.Fatal(err)
			}
			if count != len(weights) {
				t.Errorf("expected %d edges, got %d", len(weights), count)
			}
		},
	)
	t.Run(
		"OrderBy", func(t *testing.T) {
			results, err := driver.Edge[subscribesTo](db).
				OrderBy("weight", driver.Desc).
				Find()
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != len(weights) {
				t.Fatalf("expected %d edges, got %d", len(weights), len(results))
			}
			for i, result := range results {
				expected := weights[len(weights)-i-1]
				if result.Weight != expected {
					t.Errorf("expected weight %v at index %d, got %v", expected, i, result.Weight)
				}
			}
		},
	)
	t.Run(
		"Take", func(t *testing.T) {
			result, err := driver.Edge[subscribesTo](db).
				Where("weight", comparator.EQ, 2.0).
				Take()
			if err != nil {
				t.Fatal(err)
			}
			if result.Weight != 2 {
				t.Errorf("expected weight 2, got %v", result.Weight)
			}
		},
	)
	t.Run(
		"TakeNotFound", func(t *testing.T) {
			_, err := driver.Edge[subscribesTo](db).
				Where("weight", comparator.GT, 100.0).
				Take()
			if !errors.Is(err, gsmtypes.ErrNotFound) {
				t.Errorf("expected ErrNotFound, got %v", err)
			}
		},
	)
	t.Run(
		"Limit", func(t *testing.T) {
			results, err := driver.Edge[subscribesTo](db).Limit(2).Find()
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 2 {
				t.Errorf("expected 2 edges, got %d", len(results))
			}
		},
	)
	t.Run(
		"Delete", func(t *testing.T) {
			err := driver.Edge[subscribesTo](db).
				Where("weight", comparator.GT, 1.0).
				Delete()
			if err != nil {
				t.Fatal(err)
			}
			count, err := driver.Edge[subscribesTo](db).Count()
			if err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Errorf("expected 1 edge after delete, got %d", count)
			}
		},
	)
}

func TestEdgeUpdatesAndRemoveProperty(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(cleanDB)
	person, topic := seedEdgeEndpoints(t, db)

	sub := subscribesTo{Weight: 1, Notes: "original"}
	if err := driver.CreateEdge(db, &sub, &person, &topic); err != nil {
		t.Fatal(err)
	}

	err := driver.Edge[subscribesTo](db).
		Where("weight", comparator.EQ, 1.0).
		Updates(map[string]any{"notes": "bulk-updated", "weight": 9.0})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := driver.Edge[subscribesTo](db).ID(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Notes != "bulk-updated" {
		t.Errorf("expected notes %q, got %q", "bulk-updated", loaded.Notes)
	}
	if loaded.Weight != 9 {
		t.Errorf("expected weight 9, got %v", loaded.Weight)
	}
	if !loaded.LastModified.After(sub.LastModified) {
		t.Error("Updates should refresh the edge's last_modified property")
	}

	err = driver.Edge[subscribesTo](db).
		Where("weight", comparator.EQ, 9.0).
		RemoveProperty("notes")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err = driver.Edge[subscribesTo](db).ID(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Notes != "" {
		t.Errorf("expected notes removed, got %q", loaded.Notes)
	}

	t.Run(
		"UnknownProperty", func(t *testing.T) {
			err := driver.Edge[subscribesTo](db).Update("unknown_property", 1)
			if err == nil {
				t.Error("expected error updating unknown property")
			}
		},
	)
}

type hookEdge struct {
	gsmtypes.Edge
	Name string `gremlin:"name"`
	// Loaded is set by AfterFind and never persisted.
	Loaded bool `gremlin:"-"`

	beforeCreateCalls int
	afterCreateCalls  int
	beforeUpdateCalls int
	afterUpdateCalls  int
}

func (h *hookEdge) BeforeCreate(_ *driver.GremlinDriver) error {
	h.beforeCreateCalls++
	return nil
}

func (h *hookEdge) AfterCreate(_ *driver.GremlinDriver) error {
	h.afterCreateCalls++
	return nil
}

func (h *hookEdge) BeforeUpdate(_ *driver.GremlinDriver) error {
	h.beforeUpdateCalls++
	return nil
}

func (h *hookEdge) AfterUpdate(_ *driver.GremlinDriver) error {
	h.afterUpdateCalls++
	return nil
}

func (h *hookEdge) AfterFind(_ *driver.GremlinDriver) error {
	h.Loaded = true
	return nil
}

func TestEdgeHooks(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(cleanDB)
	person, topic := seedEdgeEndpoints(t, db)

	edge := hookEdge{Name: "hooked"}
	if err := driver.CreateEdge(db, &edge, &person, &topic); err != nil {
		t.Fatal(err)
	}
	if edge.beforeCreateCalls != 1 || edge.afterCreateCalls != 1 {
		t.Errorf(
			"expected create hooks to run once, got before=%d after=%d",
			edge.beforeCreateCalls, edge.afterCreateCalls,
		)
	}

	edge.Name = "hooked-updated"
	if err := driver.SaveEdge(db, &edge, &person, &topic); err != nil {
		t.Fatal(err)
	}
	if edge.beforeUpdateCalls != 1 || edge.afterUpdateCalls != 1 {
		t.Errorf(
			"expected update hooks to run once, got before=%d after=%d",
			edge.beforeUpdateCalls, edge.afterUpdateCalls,
		)
	}

	loaded, err := driver.Edge[hookEdge](db).ID(edge.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Loaded {
		t.Error("expected AfterFind hook to run on edge load")
	}
}

type edgeWithCustomLabel struct {
	gsmtypes.Edge
	Note string `gremlin:"note"`
}

func (*edgeWithCustomLabel) Label() string {
	return "a custom edge label"
}

func TestEdgeCustomLabel(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(cleanDB)
	person, topic := seedEdgeEndpoints(t, db)

	edge := edgeWithCustomLabel{Note: "labeled"}
	if err := driver.CreateEdge(db, &edge, &person, &topic); err != nil {
		t.Fatal(err)
	}
	label, err := db.G().E(edge.ID).Label().Next()
	if err != nil {
		t.Fatal(err)
	}
	if label.GetString() != "a custom edge label" {
		t.Errorf("expected custom label, got %q", label.GetString())
	}
	loaded, err := driver.Edge[edgeWithCustomLabel](db).Take()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Note != edge.Note {
		t.Errorf("expected note %q, got %q", edge.Note, loaded.Note)
	}
}

func TestCreateEdgeCustomIDGenerator(t *testing.T) {
	t.Cleanup(cleanDB)
	personID := uuid.New()
	topicID := uuid.New()
	edgeID := uuid.New()
	generatedIDs := []string{personID.String(), topicID.String(), edgeID.String()}
	nextID := 0
	db, err := driver.Open(
		DbURL, driver.Config{
			Driver: dbDriver,
			IDGenerator: func() any {
				id := generatedIDs[nextID]
				nextID++
				return id
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	person := edgeTestPerson{Name: "carol"}
	if err = driver.Create(db, &person); err != nil {
		t.Fatal(err)
	}
	topic := edgeTestTopic{Title: "custom ids"}
	if err = driver.Create(db, &topic); err != nil {
		t.Fatal(err)
	}

	sub := subscribesTo{Weight: 1}
	if err = driver.CreateEdge(db, &sub, &person, &topic); err != nil {
		t.Fatal(err)
	}
	if sub.ID != edgeID {
		t.Errorf("expected edge ID %v, got %v", edgeID, sub.ID)
	}
	loaded, err := driver.Edge[subscribesTo](db).ID(edgeID.String())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Weight != sub.Weight {
		t.Errorf("expected weight %v, got %v", sub.Weight, loaded.Weight)
	}
}

func TestEdgeQueryFromTo(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(cleanDB)

	alice := edgeTestPerson{Name: "alice"}
	bob := edgeTestPerson{Name: "bob"}
	topicOne := edgeTestTopic{Title: "one"}
	topicTwo := edgeTestTopic{Title: "two"}
	for _, v := range []any{&alice, &bob} {
		if err := driver.Create(db, v.(*edgeTestPerson)); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []any{&topicOne, &topicTwo} {
		if err := driver.Create(db, v.(*edgeTestTopic)); err != nil {
			t.Fatal(err)
		}
	}
	// alice -> one (1), alice -> two (2), bob -> one (3)
	seededEdges := []struct {
		from   *edgeTestPerson
		to     *edgeTestTopic
		weight float64
	}{
		{&alice, &topicOne, 1},
		{&alice, &topicTwo, 2},
		{&bob, &topicOne, 3},
	}
	for _, seed := range seededEdges {
		sub := subscribesTo{Weight: seed.weight}
		if err := driver.CreateEdge(db, &sub, seed.from, seed.to); err != nil {
			t.Fatal(err)
		}
	}
	// A differently-labeled edge between the same vertices must not leak
	// into subscribesTo results.
	other := edgeWithCustomLabel{Note: "noise"}
	if err := driver.CreateEdge(db, &other, &alice, &topicOne); err != nil {
		t.Fatal(err)
	}

	t.Run(
		"FromVertexStruct", func(t *testing.T) {
			results, err := driver.Edge[subscribesTo](db).From(&alice).Find()
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 2 {
				t.Errorf("expected 2 edges from alice, got %d", len(results))
			}
		},
	)
	t.Run(
		"FromRawID", func(t *testing.T) {
			results, err := driver.Edge[subscribesTo](db).From(alice.ID).Find()
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 2 {
				t.Errorf("expected 2 edges from alice, got %d", len(results))
			}
		},
	)
	t.Run(
		"ToVertex", func(t *testing.T) {
			results, err := driver.Edge[subscribesTo](db).To(&topicOne).Find()
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 2 {
				t.Errorf("expected 2 edges to topic one, got %d", len(results))
			}
		},
	)
	t.Run(
		"FromAndTo", func(t *testing.T) {
			result, err := driver.Edge[subscribesTo](db).From(&alice).To(&topicOne).Take()
			if err != nil {
				t.Fatal(err)
			}
			if result.Weight != 1 {
				t.Errorf("expected weight 1 for alice->one, got %v", result.Weight)
			}
			count, err := driver.Edge[subscribesTo](db).From(&alice).To(&topicOne).Count()
			if err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Errorf("expected 1 edge between alice and topic one, got %d", count)
			}
		},
	)
	t.Run(
		"FromWithWhere", func(t *testing.T) {
			results, err := driver.Edge[subscribesTo](db).
				From(&alice).
				Where("weight", comparator.GT, 1.0).
				Find()
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 1 || results[0].Weight != 2 {
				t.Errorf("expected single weight-2 edge, got %v", results)
			}
		},
	)
	t.Run(
		"Errors", func(t *testing.T) {
			if _, err := driver.Edge[edgeTestPerson](db).Find(); err == nil ||
				!strings.Contains(err.Error(), "does not implement EdgeType") {
				t.Errorf("expected Edge on vertex type error, got %v", err)
			}
			if _, err := driver.Model[edgeTestPerson](db).From(&alice).Find(); err == nil ||
				!strings.Contains(err.Error(), "only supported on edge queries") {
				t.Errorf("expected vertex-query error, got %v", err)
			}
			unsaved := edgeTestPerson{Name: "unsaved"}
			if _, err := driver.Edge[subscribesTo](db).From(&unsaved).Find(); err == nil ||
				!strings.Contains(err.Error(), "vertex has no id") {
				t.Errorf("expected missing id error, got %v", err)
			}
			if _, err := driver.Edge[subscribesTo](db).From(nil).Find(); err == nil ||
				!strings.Contains(err.Error(), "endpoint is nil") {
				t.Errorf("expected nil endpoint error, got %v", err)
			}
			if _, err := driver.Edge[subscribesTo](db).
				PreQuery(db.G().V()).
				From(&alice).
				Find(); err == nil ||
				!strings.Contains(err.Error(), "cannot be combined with PreQuery") {
				t.Errorf("expected PreQuery combination error, got %v", err)
			}
			if _, err := driver.Edge[subscribesTo](db).
				From(&alice).
				PreQuery(db.G().V()).
				Find(); err == nil ||
				!strings.Contains(err.Error(), "cannot be combined with From/To") {
				t.Errorf("expected From/To combination error, got %v", err)
			}
		},
	)
	t.Run(
		"FromWithDelete", func(t *testing.T) {
			if err := driver.Edge[subscribesTo](db).From(&bob).Delete(); err != nil {
				t.Fatal(err)
			}
			count, err := driver.Edge[subscribesTo](db).Count()
			if err != nil {
				t.Fatal(err)
			}
			if count != 2 {
				t.Errorf("expected 2 edges after deleting bob's, got %d", count)
			}
		},
	)
}

func TestEdgePreloadNotSupported(t *testing.T) {
	db := openEdgeTestDB(t)
	_, err := driver.Edge[subscribesTo](db).Preload("Topics").Find()
	if err == nil || !strings.Contains(err.Error(), "not supported on edge queries") {
		t.Errorf("expected preload error for edge query, got %v", err)
	}
}

// TestCreateEdgeFeedsPreload verifies edges created through the ORM are the
// same edges the vertex Preload API traverses.
func TestCreateEdgeFeedsPreload(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(cleanDB)

	person := edgeTestPerson{Name: "dave"}
	if err := driver.Create(db, &person); err != nil {
		t.Fatal(err)
	}
	titles := []string{"gremlin", "tinkerpop"}
	for _, title := range titles {
		topic := edgeTestTopic{Title: title}
		if err := driver.Create(db, &topic); err != nil {
			t.Fatal(err)
		}
		sub := subscribesTo{Weight: 1}
		if err := driver.CreateEdge(db, &sub, &person, &topic); err != nil {
			t.Fatal(err)
		}
	}

	loaded, err := driver.Model[edgeTestPerson](db).Preload("Topics").Take()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Topics) != len(titles) {
		t.Fatalf("expected %d preloaded topics, got %d", len(titles), len(loaded.Topics))
	}
}
