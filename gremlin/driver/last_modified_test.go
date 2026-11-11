package driver_test

import (
	"testing"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/comparator"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

// testLegacyVertex mirrors a legacy schema: it implements VertexType manually,
// has no timestamp properties in its gremlin tags, and disables automatic
// last-modified tracking so the driver never writes unexpected properties.
type testLegacyVertex struct {
	ID   any    `json:"id"   gremlin:"id"`
	Name string `json:"name" gremlin:"name"`
	Sort int    `json:"sort" gremlin:"sort"`

	lastModified time.Time
}

func (t *testLegacyVertex) GetVertexID() any                 { return t.ID }
func (t *testLegacyVertex) GetVertexLastModified() time.Time { return t.lastModified }
func (t *testLegacyVertex) GetVertexCreatedAt() time.Time    { return time.Time{} }
func (t *testLegacyVertex) SetVertexID(id any)               { t.ID = id }
func (t *testLegacyVertex) SetVertexLastModified(ts time.Time) {
	t.lastModified = ts
}
func (t *testLegacyVertex) SetVertexCreatedAt(time.Time) {}

// LastModifiedProperty disables automatic last-modified tracking.
func (t *testLegacyVertex) LastModifiedProperty() string { return "" }

// testVertexCustomUpdatedAt tracks its update time in a renamed property.
type testVertexCustomUpdatedAt struct {
	gsmtypes.Vertex
	Name      string    `json:"name"       gremlin:"name"`
	UpdatedAt time.Time `json:"updated_at" gremlin:"updated_at"`
}

// LastModifiedProperty renames the automatically tracked property.
func (t *testVertexCustomUpdatedAt) LastModifiedProperty() string { return "updated_at" }

func TestLastModifiedPropertyResolution(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "DefaultsToLastModified",
			got:  driver.LastModifiedPropertyForTest[testVertexForUtils](),
			want: gsmtypes.LastModified,
		},
		{
			name: "DisabledViaInterface",
			got:  driver.LastModifiedPropertyForTest[testLegacyVertex](),
			want: "",
		},
		{
			name: "DisabledViaInterfacePointerType",
			got:  driver.LastModifiedPropertyForTest[*testLegacyVertex](),
			want: "",
		},
		{
			name: "RenamedViaInterface",
			got:  driver.LastModifiedPropertyForTest[testVertexCustomUpdatedAt](),
			want: "updated_at",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				if tt.got != tt.want {
					t.Errorf("Expected property %q, got %q", tt.want, tt.got)
				}
			},
		)
	}
}

// countProperty returns how many values of the given property exist on
// vertices with the given label, bypassing struct mapping entirely.
func countProperty(db *driver.GremlinDriver, label, property string) (int, error) {
	result, err := db.G().V().HasLabel(label).Properties(property).Count().Next()
	if err != nil {
		return 0, err
	}
	return result.GetInt()
}

func TestLastModifiedTrackingDisabled(t *testing.T) {
	t.Cleanup(func() { cleanDB(t) })
	db, err := driver.Open(
		DbURL, driver.Config{
			Driver: dbDriver,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	label := driver.GetLabel[testLegacyVertex]()

	v := testLegacyVertex{Name: "legacy", Sort: 1}
	err = driver.Create(db, &v)
	if err != nil {
		t.Fatal(err)
	}
	if !v.GetVertexLastModified().IsZero() {
		t.Error("Create should not stamp last modified when tracking is disabled")
	}

	v.Sort = 2
	err = driver.Save(db, &v)
	if err != nil {
		t.Fatal(err)
	}
	if !v.GetVertexLastModified().IsZero() {
		t.Error("Save should not stamp last modified when tracking is disabled")
	}

	err = driver.Model[testLegacyVertex](db).
		Where("name", comparator.EQ, "legacy").
		Update("sort", 3)
	if err != nil {
		t.Fatal(err)
	}

	count, err := countProperty(db, label, gsmtypes.LastModified)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf(
			"Expected no %s property on legacy vertices, found %d",
			gsmtypes.LastModified, count,
		)
	}

	model, err := driver.Model[testLegacyVertex](db).Where("name", comparator.EQ, "legacy").Take()
	if err != nil {
		t.Fatal(err)
	}
	if model.Sort != 3 {
		t.Errorf("Expected sort %d, got %d", 3, model.Sort)
	}
}

func TestLastModifiedTrackingRenamedProperty(t *testing.T) {
	t.Cleanup(func() { cleanDB(t) })
	db, err := driver.Open(
		DbURL, driver.Config{
			Driver: dbDriver,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	v := testVertexCustomUpdatedAt{Name: "renamed"}
	err = driver.Create(db, &v)
	if err != nil {
		t.Fatal(err)
	}

	preUpdate, err := driver.Model[testVertexCustomUpdatedAt](db).
		Where("name", comparator.EQ, "renamed").
		Take()
	if err != nil {
		t.Fatal(err)
	}

	err = driver.Model[testVertexCustomUpdatedAt](db).
		Where("name", comparator.EQ, "renamed").
		Updates(map[string]any{"name": "renamed-updated"})
	if err != nil {
		t.Fatal(err)
	}

	model, err := driver.Model[testVertexCustomUpdatedAt](db).
		Where("name", comparator.EQ, "renamed-updated").
		Take()
	if err != nil {
		t.Fatal(err)
	}
	if preUpdate.UpdatedAt.Equal(model.UpdatedAt) {
		t.Error("Expected updated_at to be refreshed by Updates")
	}
	if !preUpdate.LastModified.Equal(model.LastModified) {
		t.Errorf(
			"Expected last_modified to be untouched by Updates, was %v now %v",
			preUpdate.LastModified, model.LastModified,
		)
	}
}

// edgeNoTracking embeds gsmtypes.Edge with automatic last-modified tracking
// disabled. The embedded gremlin tag is the write path that used to leak a
// zero timestamp to the store on every edge save even with tracking off.
type edgeNoTracking struct {
	gsmtypes.Edge
	Weight float64 `gremlin:"weight"`
}

// LastModifiedProperty disables automatic last-modified tracking.
func (e *edgeNoTracking) LastModifiedProperty() string { return "" }

// vertexNoTracking is the embedded-vertex counterpart of edgeNoTracking:
// tracking disabled while the embedded tag still maps LastModified.
type vertexNoTracking struct {
	gsmtypes.Vertex
	Name string `gremlin:"name"`
}

// LastModifiedProperty disables automatic last-modified tracking.
func (v *vertexNoTracking) LastModifiedProperty() string { return "" }

// countEdgeProperty returns how many values of the given property exist on
// edges with the given label, bypassing struct mapping entirely.
func countEdgeProperty(db *driver.GremlinDriver, label, property string) (int, error) {
	result, err := db.G().E().HasLabel(label).Properties(property).Count().Next()
	if err != nil {
		return 0, err
	}
	return result.GetInt()
}

// elementPropertyTime reads a single time-valued property from the element
// the traversal selects, bypassing struct mapping entirely.
func elementPropertyTime(
	t *testing.T, traversal *gremlingo.GraphTraversal, property string,
) time.Time {
	t.Helper()
	result, err := traversal.Values(property).Next()
	if err != nil {
		t.Fatalf("read property %s: %v", property, err)
	}
	switch ts := result.GetInterface().(type) {
	case time.Time:
		return ts
	case *time.Time:
		if ts != nil {
			return *ts
		}
	}
	t.Fatalf("property %s is %T, want a time", property, result.GetInterface())
	return time.Time{}
}

func TestEdgeLastModifiedTrackingDisabled(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(func() { cleanDB(t) })
	person, topic := seedEdgeEndpoints(t, db)
	label := driver.GetLabel[edgeNoTracking]()

	edge := edgeNoTracking{Weight: 2.5}
	if err := driver.SaveEdge(db, &edge, &person, &topic); err != nil {
		t.Fatal(err)
	}
	if !edge.LastModified.IsZero() {
		t.Errorf(
			"SaveEdge create should not stamp last modified when tracking is disabled, got %v",
			edge.LastModified,
		)
	}
	count, err := countEdgeProperty(db, label, gsmtypes.LastModified)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf(
			"Expected no %s property on %s edges, found %d",
			gsmtypes.LastModified, label, count,
		)
	}
	// The skip must stay scoped to last_modified: created_at is still stamped.
	if edge.CreatedAt.IsZero() {
		t.Error("edge CreatedAt should still be stamped on create")
	}
	created, err := countEdgeProperty(db, label, gsmtypes.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Errorf("Expected 1 %s property on %s edges, found %d", gsmtypes.CreatedAt, label, created)
	}

	edge.Weight = 3.5
	if saveErr := driver.SaveEdge(db, &edge, &person, &topic); saveErr != nil {
		t.Fatal(saveErr)
	}
	if !edge.LastModified.IsZero() {
		t.Errorf(
			"SaveEdge update should not stamp last modified when tracking is disabled, got %v",
			edge.LastModified,
		)
	}
	count, err = countEdgeProperty(db, label, gsmtypes.LastModified)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf(
			"Expected no %s property after SaveEdge update, found %d",
			gsmtypes.LastModified, count,
		)
	}

	if updateErr := driver.Edge[edgeNoTracking](db).
		Where("weight", comparator.EQ, 3.5).
		Update("weight", 4.5); updateErr != nil {
		t.Fatal(updateErr)
	}
	count, err = countEdgeProperty(db, label, gsmtypes.LastModified)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf(
			"Expected no %s property after Update, found %d",
			gsmtypes.LastModified, count,
		)
	}
}

// storedLastModified is an explicit, millisecond-exact timestamp so the
// value round-trips through the server's Date serialization unchanged.
var storedLastModified = time.Date(2020, 5, 6, 7, 8, 9, 0, time.UTC)

func TestEdgeLastModifiedDisabledExplicitValuePersisted(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(func() { cleanDB(t) })
	person, topic := seedEdgeEndpoints(t, db)

	edge := edgeNoTracking{Weight: 1.0}
	edge.SetEdgeLastModified(storedLastModified)
	if err := driver.SaveEdge(db, &edge, &person, &topic); err != nil {
		t.Fatal(err)
	}
	if got := elementPropertyTime(
		t,
		db.G().E(edge.ID),
		gsmtypes.LastModified,
	); !got.Equal(
		storedLastModified,
	) {
		t.Errorf("Expected stored last_modified %v, got %v", storedLastModified, got)
	}

	edge.Weight = 2.0
	if err := driver.SaveEdge(db, &edge, &person, &topic); err != nil {
		t.Fatal(err)
	}
	if got := elementPropertyTime(
		t,
		db.G().E(edge.ID),
		gsmtypes.LastModified,
	); !got.Equal(
		storedLastModified,
	) {
		t.Errorf(
			"Expected last_modified to keep the explicit value %v after update, got %v",
			storedLastModified, got,
		)
	}
}

func TestEmbeddedVertexLastModifiedTrackingDisabled(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(func() { cleanDB(t) })
	label := driver.GetLabel[vertexNoTracking]()

	v := vertexNoTracking{Name: "silent"}
	if err := driver.Create(db, &v); err != nil {
		t.Fatal(err)
	}
	if !v.LastModified.IsZero() {
		t.Errorf(
			"Create should not stamp last modified when tracking is disabled, got %v",
			v.LastModified,
		)
	}
	count, err := countProperty(db, label, gsmtypes.LastModified)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf(
			"Expected no %s property on %s vertices, found %d",
			gsmtypes.LastModified, label, count,
		)
	}

	v.Name = "still-silent"
	if saveErr := driver.Save(db, &v); saveErr != nil {
		t.Fatal(saveErr)
	}
	count, err = countProperty(db, label, gsmtypes.LastModified)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf(
			"Expected no %s property after Save, found %d",
			gsmtypes.LastModified, count,
		)
	}

	// An explicitly set value stays persisted through the gremlin tag.
	v.SetVertexLastModified(storedLastModified)
	if saveErr := driver.Save(db, &v); saveErr != nil {
		t.Fatal(saveErr)
	}
	if got := elementPropertyTime(
		t,
		db.G().V(v.ID),
		gsmtypes.LastModified,
	); !got.Equal(
		storedLastModified,
	) {
		t.Errorf(
			"Expected stored last_modified %v, got %v",
			storedLastModified, got,
		)
	}
}

// renamedSetterVertex is the README's rename recipe: tracking is renamed to
// updated_at and SetVertexLastModified is overridden to fill that custom
// field, leaving the embedded gsmtypes.Vertex LastModified zero forever.
// That zero must never reach the store.
type renamedSetterVertex struct {
	gsmtypes.Vertex
	Name      string    `gremlin:"name"`
	UpdatedAt time.Time `gremlin:"updated_at"`
}

// LastModifiedProperty renames the automatically tracked property.
func (r *renamedSetterVertex) LastModifiedProperty() string { return "updated_at" }

// SetVertexLastModified routes the driver's stamp into the custom field.
func (r *renamedSetterVertex) SetVertexLastModified(ts time.Time) { r.UpdatedAt = ts }

func TestRenamedLastModifiedZeroEmbeddedFieldSkipped(t *testing.T) {
	db := openEdgeTestDB(t)
	t.Cleanup(func() { cleanDB(t) })
	label := driver.GetLabel[renamedSetterVertex]()

	v := renamedSetterVertex{Name: "article"}
	if err := driver.Create(db, &v); err != nil {
		t.Fatal(err)
	}
	if v.UpdatedAt.IsZero() {
		t.Fatal("Create should stamp the renamed tracked field")
	}
	// The skip must stay scoped to last_modified: created_at is still stamped.
	created, err := countProperty(db, label, gsmtypes.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Errorf(
			"Expected 1 %s property on %s vertices, found %d",
			gsmtypes.CreatedAt, label, created,
		)
	}
	// The stamp lands on the custom field, through its gremlin tag.
	// java.util.Date round-trips at millisecond precision.
	if got := elementPropertyTime(t, db.G().V(v.ID), "updated_at"); !got.Equal(
		v.UpdatedAt.Truncate(time.Millisecond),
	) {
		t.Errorf("Expected stored updated_at %v, got %v", v.UpdatedAt, got)
	}
	// The superseded embedded zero field must not be persisted.
	count, err := countProperty(db, label, gsmtypes.LastModified)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf(
			"Expected no %s property on %s vertices, found %d",
			gsmtypes.LastModified, label, count,
		)
	}

	v.Name = "edited"
	if saveErr := driver.Save(db, &v); saveErr != nil {
		t.Fatal(saveErr)
	}
	count, err = countProperty(db, label, gsmtypes.LastModified)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf(
			"Expected no %s property on %s vertices after Save, found %d",
			gsmtypes.LastModified, label, count,
		)
	}
	if got := elementPropertyTime(t, db.G().V(v.ID), "updated_at"); !got.Equal(
		v.UpdatedAt.Truncate(time.Millisecond),
	) {
		t.Errorf("Expected refreshed updated_at %v, got %v", v.UpdatedAt, got)
	}
}
