package driver_test

import (
	"testing"

	"github.com/jbrusegaard/graph-struct-manager/comparator"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

type testVertex struct {
	gsmtypes.Vertex
	Name string `json:"name" gremlin:"name"`
}

const DbURL = "ws://localhost:8182"

func TestDriverConnections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "ValidConnection",
			url:     DbURL,
			wantErr: false,
		},
		{
			name:    "InvalidURL",
			url:     "invalid-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				db, err := driver.Open(tt.url, dbDriver)
				if (err != nil) != tt.wantErr {
					t.Errorf("Open() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if db != nil {
					defer db.Close()
				}
			},
		)
	}
}

func TestDriverTable(t *testing.T) {
	db, err := driver.Open(DbURL, dbDriver)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	table := db.Label("test_vertex")
	if table == nil {
		t.Fatal("Table should not be nil")
	}
	if _, err := table.Limit(1).ToList(); err != nil {
		t.Errorf("Label() query should not error, got %v", err)
	}
}

func TestDriverModel(t *testing.T) {
	db, err := driver.Open(DbURL, dbDriver)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	model := driver.Model[testVertex](db)
	if model == nil {
		t.Fatal("Model should not be nil")
	}
}

func TestDriverWhere(t *testing.T) {
	db, err := driver.Open(DbURL, dbDriver)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	model := driver.Where[testVertex](db, "name", comparator.EQ, "test")
	if model == nil {
		t.Fatal("Model should not be nil")
	}
}

func TestDriverSave(t *testing.T) {
	db, err := driver.Open(DbURL, dbDriver)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	testV := &testVertex{
		Name: "pre-test",
	}
	err = driver.Create(db, testV)
	if err != nil {
		t.Error(err)
	}
	testV.Name = "post-test"
	err = driver.Save(db, testV)
	if err != nil {
		return
	}
	vertex, err := driver.Model[testVertex](db).ID(testV.ID)
	if err != nil {
		return
	}
	if vertex.Name != testV.Name {
		t.Errorf("vertex name should be %s, got %s", testV.Name, vertex.Name)
	}
}
