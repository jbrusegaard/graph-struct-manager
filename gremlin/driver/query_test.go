package driver_test

import (
	"errors"
	"slices"
	"testing"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/google/uuid"
	"github.com/jbrusegaard/graph-struct-manager/comparator"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

var dbDriver = driver.Neptune

func seedData(db *driver.GremlinDriver, data []testVertexForUtils) error {
	for _, d := range data {
		err := driver.Create(db, &d)
		if err != nil {
			return err
		}
	}
	return nil
}

func cleanDB() {
	db, _ := driver.Open(
		DbURL, driver.Config{
			Driver: dbDriver,
		},
	)
	<-db.G().V().Drop().Iterate()
}

func TestQuery(t *testing.T) {
	db, err := driver.Open(
		DbURL, driver.Config{
			Driver: dbDriver,
		},
	)
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
		},
	}

	t.Run(
		"TestFindWhereFirst", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			model := driver.Model[testVertexForUtils](db)
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
		Order driver.GremlinOrder
	}{
		{Name: "TestFindNoWhereOrderAsc", Order: driver.Asc},
		{Name: "TestFindWhereOrderDesc", Order: driver.Desc},
	}
	for _, orderTest := range orderTests {
		t.Run(
			orderTest.Name, func(t *testing.T) {
				t.Cleanup(cleanDB)
				err = seedData(db, seededData)
				if err != nil {
					t.Error(err)
				}
				model := driver.Model[testVertexForUtils](db)
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
					case driver.Asc:
						idx = i
					case driver.Desc:
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
			model := driver.Model[testVertexForUtils](
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
		"TestQueryPreQuery", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			preQuery := db.G().V().Has("name", "second")
			result, err := driver.Model[testVertexForUtils](
				db,
			).PreQuery(preQuery).
				Where("sort", comparator.GT, 1).
				Take()
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
			err := driver.Model[testVertexForUtils](db).Limit(1).Delete()
			if err != nil {
				t.Error(err)
			}
			count, err := driver.Model[testVertexForUtils](db).Count()
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
			model, err := driver.Model[testVertexForUtils](db).Take()
			if err != nil {
				t.Error(err)
			}
			result, err := driver.Model[testVertexForUtils](db).ID(model.ID)
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
			err = driver.Model[testVertexForUtils](db).Update("badField", "badValue")
			if err == nil {
				t.Error("Expected error")
			}
		},
	)
	t.Run(
		"Test Update query with existing slice property", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			err = driver.Model[testVertexForUtils](db).Where("name", comparator.EQ, "second").Update(
				"listTest", []string{"this is new property value", "test"},
			)
			if err != nil {
				t.Error("error updating property", err)
			}
			model, err := driver.Model[testVertexForUtils](db).Where("name", comparator.EQ, "second").Take()
			if err != nil {
				t.Error(err)
			}
			if !slices.Contains(model.ListTest, "this is new property value") || !slices.Contains(
				model.ListTest, "test",
			) {
				t.Errorf(
					"Expected %s and %s, got %s and %s", "this is new property value", "test", model.ListTest[0],
					model.ListTest[1],
				)
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
			preUpdateModel, err := driver.Model[testVertexForUtils](
				db,
			).Where("name", comparator.EQ, "first").
				Take()
			if err != nil {
				t.Error(err)
			}
			err = driver.Model[testVertexForUtils](
				db,
			).Where("name", comparator.EQ, "first").
				Update("name", "fourth")
			if err != nil {
				t.Error(err)
			}
			model, err := driver.Model[testVertexForUtils](
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
			model, err := driver.Model[testVertexForUtils](db).Where("name", comparator.EQ, "first").Take()
			if err != nil {
				t.Error(err)
			}
			model.Name = "fifth"
			model.Sort = 5
			err = driver.Save(db, &model)
			if err != nil {
				t.Error(err)
			}
			model, err = driver.Model[testVertexForUtils](db).Where("name", comparator.EQ, "fifth").Take()
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
			model, err := driver.Model[testVertexForUtils](db).Where("name", comparator.EQ, "first").Take()
			if err != nil {
				t.Error(err)
			}
			model.Name = "fifth"
			model.Sort = 5
			err = driver.Save(db, &model)
			if err != nil {
				t.Error(err)
			}
			model, err = driver.Model[testVertexForUtils](db).Where("name", comparator.EQ, "fifth").Take()
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
			models, err := driver.Model[testVertexForUtils](db).Find()
			if err != nil {
				t.Error(err)
			}
			for _, model := range models {
				mdl, err := driver.Model[testVertexForUtils](db).IDs(model.ID).Take()
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
			model := driver.Model[testVertexForUtils](db).AddSubTraversals(
				map[string]*gremlingo.GraphTraversal{
					"subTraversalTest":  gremlingo.T__.Constant("test123"),
					"subTraversalTest2": gremlingo.T__.Constant(123),
				},
			)
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
	t.Run(
		"TestCustomID", func(t *testing.T) {
			t.Cleanup(cleanDB)

			testID, err := uuid.NewRandom()
			if err != nil {
				t.Error(err)
			}

			customDbIDGenerator, _ := driver.Open(
				DbURL, driver.Config{
					Driver: dbDriver,
					IDGenerator: func() any {
						return testID.String()
					},
				},
			)

			data := testVertexForUtils{
				Name: "test",
			}
			err = driver.Create(customDbIDGenerator, &data)
			if err != nil {
				t.Error(err)
			}
			model, err := driver.Model[testVertexForUtils](customDbIDGenerator).ID(testID.String())
			if err != nil {
				t.Error(err)
			}
			if model.ID != testID {
				t.Errorf("Expected %s result, got %s", testID, model.ID)
			}
		},
	)
	t.Run(
		"Test Range Query", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}

			results, err := driver.Model[testVertexForUtils](db).Range(0, 10).Find()
			if err != nil {
				t.Error(err)
			}
			if len(results) != len(seededData) {
				t.Errorf("Expected %d results, got %d", len(seededData), len(results))
			}
			results2, err := driver.Model[testVertexForUtils](
				db,
			).OrderBy("sort", driver.Asc).
				Range(2, 10).
				Find()
			if err != nil {
				t.Error(err)
			}
			if results2[0].Name != seededData[2].Name {
				t.Errorf("Expected %s result, got %s", seededData[2].Name, results2[0].Name)
			}
		},
	)
	t.Run(
		"Test Range Query with 2 as lower bound", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}

			results, err := driver.Model[testVertexForUtils](db).Range(2, 10).Find()
			if err != nil {
				t.Error(err)
			}
			if len(results) != len(seededData)-2 {
				t.Errorf("Expected %d results, got %d", len(seededData)-2, len(results))
			}
		},
	)
	t.Run(
		"Test Range with offset already set", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			results, err := driver.Model[testVertexForUtils](db).Offset(1).Range(0, 10).Find()
			if err != nil {
				t.Error(err)
			}
			if len(results) != len(seededData)-1 {
				t.Errorf("Expected %d results, got %d", len(seededData)-1, len(results))
			}
		},
	)
	t.Run(
		"TestQueryWhereIn", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			results, err := driver.Model[testVertexForUtils](db).
				Where("name", comparator.IN, []any{"first", "third"}).
				OrderBy("sort", driver.Asc).
				Find()
			if err != nil {
				t.Error(err)
			}
			if len(results) != 2 {
				t.Errorf("Expected 2 results, got %d", len(results))
			}
			if results[0].Name != "first" || results[1].Name != "third" {
				t.Errorf(
					"Expected first and third results, got %s and %s",
					results[0].Name,
					results[1].Name,
				)
			}
		},
	)
	t.Run(
		"TestQueryWhereWithout", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			results, err := driver.Model[testVertexForUtils](db).
				Where("name", comparator.WITHOUT, []any{"second"}).
				Find()
			if err != nil {
				t.Error(err)
			}
			if len(results) != 2 {
				t.Errorf("Expected 2 results, got %d", len(results))
			}
			for _, result := range results {
				if result.Name == "second" {
					t.Errorf("Did not expect second in results")
				}
			}
		},
	)
	t.Run(
		"TestQueryWhereContains", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			result, err := driver.Model[testVertexForUtils](db).
				Where("name", comparator.CONTAINS, "eco").
				Take()
			if err != nil {
				t.Error(err)
			}
			if result.Name != "second" {
				t.Errorf("Expected second result, got %s", result.Name)
			}
		},
	)
	t.Run(
		"TestQueryWhereID", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			model, err := driver.Model[testVertexForUtils](db).Take()
			if err != nil {
				t.Error(err)
			}
			result, err := driver.Model[testVertexForUtils](db).
				Where("id", comparator.EQ, model.ID).
				Take()
			if err != nil {
				t.Error(err)
			}
			if result.ID != model.ID {
				t.Errorf("Expected %s result, got %s", model.ID, result.ID)
			}
		},
	)
	t.Run(
		"TestQueryLimitOffset", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			results, err := driver.Model[testVertexForUtils](db).
				OrderBy("sort", driver.Asc).
				Offset(1).
				Limit(1).
				Find()
			if err != nil {
				t.Error(err)
			}
			if len(results) != 1 {
				t.Errorf("Expected 1 result, got %d", len(results))
			}
			if results[0].Name != "second" {
				t.Errorf("Expected second result, got %s", results[0].Name)
			}
		},
	)
	t.Run(
		"TestQueryIDsMultiple", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			models, err := driver.Model[testVertexForUtils](db).OrderBy("sort", driver.Asc).Find()
			if err != nil {
				t.Error(err)
			}
			results, err := driver.Model[testVertexForUtils](db).
				IDs(models[0].ID, models[2].ID).
				OrderBy("sort", driver.Asc).
				Find()
			if err != nil {
				t.Error(err)
			}
			if len(results) != 2 {
				t.Errorf("Expected 2 results, got %d", len(results))
			}
			if results[0].Name != models[0].Name || results[1].Name != models[2].Name {
				t.Errorf(
					"Expected %s and %s, got %s and %s",
					models[0].Name, models[2].Name, results[0].Name, results[1].Name,
				)
			}
		},
	)
	t.Run(
		"Test save with existing slice property", func(t *testing.T) {
			t.Cleanup(cleanDB)
			err = seedData(db, seededData)
			if err != nil {
				t.Error(err)
			}
			model, err := driver.Model[testVertexForUtils](db).Where("name", comparator.EQ, "second").Take()
			if err != nil {
				t.Error(err)
			}
			model.ListTest = []string{"Going to test the update with existing slice property", "another test"}
			err = driver.Save(db, &model)
			if err != nil {
				t.Error("error updating property", err)
			}
			updatedModel, err := driver.Model[testVertexForUtils](db).Where("name", comparator.EQ, "second").Take()
			if err != nil {
				t.Error(err)
			}
			if !slices.Contains(updatedModel.ListTest, "Going to test the update with existing slice property") || !slices.Contains(
				updatedModel.ListTest, "another test",
			) {
				t.Errorf(
					"Expected %s and %s, got %s and %s", "Going to test the update with existing slice property", "another test", updatedModel.ListTest[0], updatedModel.ListTest[1],
				)
			}
		},
	)
	t.Run("Test Custom Error", func(t *testing.T) {
		t.Cleanup(cleanDB)
		_, err := driver.Model[testVertexForUtils](db).Take()
		if !errors.Is(err, gsmtypes.ErrNotFound) {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})
	t.Run("Test Select", func(t *testing.T) {
		t.Cleanup(cleanDB)
		err = seedData(db, seededData)
		if err != nil {
			t.Error(err)
		}
		results, err := driver.Model[testVertexForUtils](db).Select("name").Find()
		if err != nil {
			t.Error(err)
		}
		if len(results) != len(seededData) {
			t.Errorf("Expected %d results, got %d", len(seededData), len(results))
		}
		for _, result := range results {
			if result.Name == "" {
				t.Errorf("Expected name to be set, got %s", result.Name)
			}
			if result.ID == "" {
				t.Errorf("Expected ID to be set, got %s", result.ID)
			}
			if result.ListTest != nil {
				t.Errorf("Expected listTest to be nil, got %v", result.ListTest)
			}
			if result.Sort != 0 {
				t.Errorf("Expected sort to be 0, got %d", result.Sort)
			}
		}
	})
	t.Run("Test Select with multiple fields", func(t *testing.T) {
		t.Cleanup(cleanDB)
		err = seedData(db, seededData)
		if err != nil {
			t.Error(err)
		}
		results, err := driver.Model[testVertexForUtils](db).Select("name", "sort").Find()
		if err != nil {
			t.Error(err)
		}
		if len(results) != len(seededData) {
			t.Errorf("Expected %d results, got %d", len(seededData), len(results))
		}
		for _, result := range results {
			if result.Name == "" || result.Sort == 0 {
				t.Errorf("Expected name and sort to be set, got %s and %d", result.Name, result.Sort)
			}
		}
	})
	t.Run("Test select with random field names", func(t *testing.T) {
		t.Cleanup(cleanDB)
		err = seedData(db, seededData)
		if err != nil {
			t.Error(err)
		}
		results, err := driver.Model[testVertexForUtils](db).Select("randomField").Find()
		if err != nil {
			t.Error(err)
		}
		if len(results) != len(seededData) {
			t.Errorf("Expected %d results, got %d", len(seededData), len(results))
		}
		for _, result := range results {
			if result.ID == "" {
				t.Errorf("Expected ID to be set, got %s", result.ID)
			}
			if result.Name != "" {
				t.Errorf("Expected name to be empty, got %s", result.Name)
			}
			if result.Sort != 0 {
				t.Errorf("Expected sort to be 0, got %d", result.Sort)
			}
			if result.ListTest != nil {
				t.Errorf("Expected listTest to be nil, got %v", result.ListTest)
			}
		}
	})
}
