package driver

import (
	"testing"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/comparator"
)

var dbDriver = Neptune

func seedData(db *GremlinDriver, data []testVertexForUtils) error {
	for _, d := range data {
		err := Create(db, &d)
		if err != nil {
			return err
		}
	}
	return nil
}

func cleanDB() {
	db, _ := Open(DbURL, dbDriver)
	<-db.g.V().Drop().Iterate()
}

func TestQuery(t *testing.T) {
	db, err := Open(DbURL, dbDriver)
	if err != nil {
		t.Fatal(err)
	}
	seededData := []testVertexForUtils{
		{
			Name:     "first",
			Sort:     1,
			ListTest: []string{"test123"},
		},
		{
			Name:     "second",
			Sort:     2,
			ListTest: []string{"test123", "test"},
		},
		{
			Name: "third",
			Sort: 3,
			MapTest: map[string]string{
				"test123": "test123",
			},
		},
	}

	t.Run(
		"TestFindWhereFirst", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			model := Model[testVertexForUtils](db)
			results, err := model.Where("name", comparator.EQ, "first").Find()
			if err != nil {
				t.Error(err)
			}
			if len(results) != 1 {
				t.Errorf("Expected 1 result, got %d", len(results))
			}
			if results[0].Name != "first" {
				t.Errorf("Expected first result, got %s", results[0].Name)
			}
		},
	)
	orderTests := []struct {
		Name  string
		Order GremlinOrder
	}{
		{Name: "TestFindNoWhereOrderAsc", Order: Asc},
		{Name: "TestFindWhereOrderDesc", Order: Desc},
	}
	for _, orderTest := range orderTests {
		t.Run(
			orderTest.Name, func(t *testing.T) {
				t.Cleanup(cleanDB)
				err = seedData(db, seededData)
				if err != nil {
					t.Error(err)
				}
				model := Model[testVertexForUtils](db)
				results, err := model.OrderBy("sort", orderTest.Order).Find()
				if err != nil {
					t.Error(err)
				}
				if len(results) != len(seededData) {
					t.Errorf("Expected %d results, got %d", len(seededData), len(results))
				}
				for i, item := range results {
					var idx int
					switch orderTest.Order {
					case Asc:
						idx = i
					case Desc:
						idx = len(results) - i - 1
					}
					if item.Name != seededData[idx].Name {
						t.Errorf("Expected %s result, got %s", seededData[i].Name, item.Name)
					}
				}
			},
		)
	}
	t.Run(
		"TestQueryWhereTraversal", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			model := Model[testVertexForUtils](
				db,
			).WhereTraversal(gremlingo.T__.Has("name", "second"))
			result, err := model.Take()
			if err != nil {
				t.Error(err)
			}
			if result.Name != "second" {
				t.Errorf("Expected second result, got %s", result.Name)
			}
		},
	)

	t.Run(
		"TestDelete", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			err := Model[testVertexForUtils](db).Limit(1).Delete()
			if err != nil {
				t.Error(err)
			}
			count, err := Model[testVertexForUtils](db).Count()
			if err != nil {
				t.Error(err)
			}
			if count != len(seededData)-1 {
				t.Errorf("Expected %d results, got %d", len(seededData)-1, count)
			}
		},
	)

	t.Run(
		"TestQueryById", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			model, err := Model[testVertexForUtils](db).Take()
			if err != nil {
				t.Error(err)
			}
			result, err := Model[testVertexForUtils](db).ID(model.ID)
			if err != nil {
				t.Error(err)
			}
			if result.Name != model.Name {
				t.Errorf("Expected %s result, got %s", model.Name, result.Name)
			}
			if result.ID != model.ID {
				t.Errorf("Expected %s result, got %s", model.ID, result.ID)
			}
			if result.Sort != model.Sort {
				t.Errorf("Expected %b result, got %b", model.Sort, result.Sort)
			}
		},
	)

	t.Run(
		"TestQueryUpdateBadInput", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			err = Model[testVertexForUtils](db).Update("badField", "badValue")
			if err == nil {
				t.Error("Expected error")
			}
		},
	)
	t.Run(
		"TestQueryUpdateSingleCardinality", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			preUpdateModel, err := Model[testVertexForUtils](
				db,
			).Where("name", comparator.EQ, "first").
				Take()
			if err != nil {
				t.Error(err)
			}
			err = Model[testVertexForUtils](
				db,
			).Where("name", comparator.EQ, "first").
				Update("name", "fourth")
			if err != nil {
				t.Error(err)
			}
			model, err := Model[testVertexForUtils](
				db,
			).Where("name", comparator.EQ, "fourth").
				Take()
			if err != nil {
				t.Error(err)
			}
			if model.Name != "fourth" {
				t.Errorf("Expected %s result, got %s", "fourth", model.Name)
			}
			if preUpdateModel.LastModified.Equal(model.LastModified) {
				t.Error("Expected last modified time to be updated")
			}
			if preUpdateModel.LastModified.Equal(model.LastModified) {
				t.Error("Expected last modified time to be updated")
			}
		},
	)
	t.Run(
		"TestSave", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			model, err := Model[testVertexForUtils](db).Where("name", comparator.EQ, "first").Take()
			if err != nil {
				t.Error(err)
			}
			model.Name = "fifth"
			model.Sort = 5
			err = Save(db, &model)
			if err != nil {
				t.Error(err)
			}
			model, err = Model[testVertexForUtils](db).Where("name", comparator.EQ, "fifth").Take()
			if err != nil {
				t.Error(err)
			}
			if model.Name != "fifth" {
				t.Errorf("Expected %s result, got %s", "fifth", model.Name)
			}
		},
	)
	t.Run(
		"TestUpdate", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			model, err := Model[testVertexForUtils](db).Where("name", comparator.EQ, "first").Take()
			if err != nil {
				t.Error(err)
			}
			model.Name = "fifth"
			model.Sort = 5
			err = Save(db, &model)
			if err != nil {
				t.Error(err)
			}
			model, err = Model[testVertexForUtils](db).Where("name", comparator.EQ, "fifth").Take()
			if err != nil {
				t.Error(err)
			}
			if model.Name != "fifth" {
				t.Errorf("Expected %s result, got %s", "fifth", model.Name)
			}
		},
	)
	t.Run(
		"TestQueryIDs", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			models, err := Model[testVertexForUtils](db).Find()
			if err != nil {
				t.Error(err)
			}
			for _, model := range models {
				mdl, err := Model[testVertexForUtils](db).IDs(model.ID).Take()
				if err != nil {
					t.Error(err)
				}
				if mdl.Name != model.Name {
					t.Errorf("Expected %s result, got %s", model.Name, mdl.Name)
				}
			}
		},
	)
	t.Run(
		"TestQueryAddSubTraversals", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			model := Model[testVertexForUtils](db).AddSubTraversals(map[string]*gremlingo.GraphTraversal{
				"subTraversalTest":  gremlingo.T__.Constant("test123"),
				"subTraversalTest2": gremlingo.T__.Constant(123),
			})
			result, err := model.Take()
			if err != nil {
				t.Error(err)
			}
			if result.SubTraversalTest != "test123" {
				t.Errorf("Expected %s result, got %s", "test123", result.SubTraversalTest)
			}
			if result.SubTraversalTest2 != 123 {
				t.Errorf("Expected %d result, got %d", 123, result.SubTraversalTest2)
			}
		},
	)
}
