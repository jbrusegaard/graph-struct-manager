package driver_test

import (
	"testing"
	"time"

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
	t.Cleanup(cleanDB)
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
	t.Cleanup(cleanDB)
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
