package driver_test

import (
	"sort"
	"testing"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/jbrusegaard/graph-struct-manager/comparator"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

type testTopic struct {
	gsmtypes.Vertex
	Title string `json:"title" gremlin:"title"`
}

type testPerson struct {
	gsmtypes.Vertex
	Name       string       `json:"name"   gremlin:"name"`
	Topics     []testTopic  `json:"topics"                gremlinEdge:"subscribed"`
	BestFriend *testPerson  `json:"bestFriend"            gremlinEdge:"best_friend,out"`
	Friends    []testPerson `json:"friends"               gremlinEdge:"friend,both"`
}

type testTopicWithSubscribers struct {
	gsmtypes.Vertex
	Title       string       `json:"title" gremlin:"title"`
	Subscribers []testPerson `json:"subscribers" gremlinEdge:"subscribed,in"`
}

func (t *testTopicWithSubscribers) Label() string {
	return "test_topic"
}

func addEdge(t *testing.T, db *driver.GremlinDriver, fromID any, label string, toID any) {
	t.Helper()
	err := <-db.G().V(fromID).AddE(label).To(gremlingo.T__.V(toID)).Iterate()
	if err != nil {
		t.Fatal(err)
	}
}

func seedPreloadData(t *testing.T, db *driver.GremlinDriver) (testPerson, []testTopic) {
	t.Helper()
	person := testPerson{Name: "alice"}
	if err := driver.Create(db, &person); err != nil {
		t.Fatal(err)
	}
	topics := []testTopic{{Title: "graphs"}, {Title: "golang"}}
	for i := range topics {
		if err := driver.Create(db, &topics[i]); err != nil {
			t.Fatal(err)
		}
		addEdge(t, db, person.ID, "subscribed", topics[i].ID)
	}
	return person, topics
}

func TestPreload(t *testing.T) {
	db, err := driver.Open(
		DbURL, driver.Config{
			Driver: dbDriver,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"TestPreloadSliceOut", func(t *testing.T) {
			t.Cleanup(cleanDB)
			_, topics := seedPreloadData(t, db)

			result, err := driver.Model[testPerson](db).Preload("Topics").Take()
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Topics) != len(topics) {
				t.Fatalf("Expected %d topics, got %d", len(topics), len(result.Topics))
			}
			sort.Slice(result.Topics, func(i, j int) bool {
				return result.Topics[i].Title < result.Topics[j].Title
			})
			if result.Topics[0].Title != "golang" || result.Topics[1].Title != "graphs" {
				t.Errorf("Expected golang and graphs, got %+v", result.Topics)
			}
			for _, topic := range result.Topics {
				if topic.ID == nil || topic.ID == "" {
					t.Errorf("Expected topic ID to be loaded, got %v", topic.ID)
				}
			}
		},
	)

	t.Run(
		"TestPreloadWithFind", func(t *testing.T) {
			t.Cleanup(cleanDB)
			seedPreloadData(t, db)
			// second person with no subscriptions
			bob := testPerson{Name: "bob"}
			if err := driver.Create(db, &bob); err != nil {
				t.Fatal(err)
			}

			results, err := driver.Model[testPerson](db).Preload("Topics").Find()
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 2 {
				t.Fatalf("Expected 2 people, got %d", len(results))
			}
			for _, person := range results {
				switch person.Name {
				case "alice":
					if len(person.Topics) != 2 {
						t.Errorf("Expected alice to have 2 topics, got %d", len(person.Topics))
					}
				case "bob":
					if len(person.Topics) != 0 {
						t.Errorf("Expected bob to have 0 topics, got %d", len(person.Topics))
					}
				}
			}
		},
	)

	t.Run(
		"TestPreloadInDirection", func(t *testing.T) {
			t.Cleanup(cleanDB)
			person, topics := seedPreloadData(t, db)

			result, err := driver.Model[testTopicWithSubscribers](db).
				Where("title", comparator.EQ, topics[0].Title).
				Preload("Subscribers").
				Take()
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Subscribers) != 1 {
				t.Fatalf("Expected 1 subscriber, got %d", len(result.Subscribers))
			}
			if result.Subscribers[0].Name != person.Name {
				t.Errorf("Expected subscriber %s, got %s", person.Name, result.Subscribers[0].Name)
			}
		},
	)

	t.Run(
		"TestPreloadPointerField", func(t *testing.T) {
			t.Cleanup(cleanDB)
			alice := testPerson{Name: "alice"}
			bob := testPerson{Name: "bob"}
			if err := driver.Create(db, &alice); err != nil {
				t.Fatal(err)
			}
			if err := driver.Create(db, &bob); err != nil {
				t.Fatal(err)
			}
			addEdge(t, db, alice.ID, "best_friend", bob.ID)

			result, err := driver.Model[testPerson](db).
				Where("name", comparator.EQ, "alice").
				Preload("BestFriend").
				Take()
			if err != nil {
				t.Fatal(err)
			}
			if result.BestFriend == nil {
				t.Fatal("Expected best friend to be loaded")
			}
			if result.BestFriend.Name != "bob" {
				t.Errorf("Expected best friend bob, got %s", result.BestFriend.Name)
			}

			// bob has no outgoing best_friend edge
			result, err = driver.Model[testPerson](db).
				Where("name", comparator.EQ, "bob").
				Preload("BestFriend").
				Take()
			if err != nil {
				t.Fatal(err)
			}
			if result.BestFriend != nil {
				t.Errorf("Expected best friend to be nil, got %+v", result.BestFriend)
			}
		},
	)

	t.Run(
		"TestPreloadBothDirection", func(t *testing.T) {
			t.Cleanup(cleanDB)
			alice := testPerson{Name: "alice"}
			bob := testPerson{Name: "bob"}
			carol := testPerson{Name: "carol"}
			for _, p := range []*testPerson{&alice, &bob, &carol} {
				if err := driver.Create(db, p); err != nil {
					t.Fatal(err)
				}
			}
			addEdge(t, db, alice.ID, "friend", bob.ID)
			addEdge(t, db, carol.ID, "friend", alice.ID)

			result, err := driver.Model[testPerson](db).
				Where("name", comparator.EQ, "alice").
				Preload("Friends").
				Take()
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Friends) != 2 {
				t.Fatalf("Expected 2 friends, got %d", len(result.Friends))
			}
		},
	)

	t.Run(
		"TestPreloadMultipleFields", func(t *testing.T) {
			t.Cleanup(cleanDB)
			person, _ := seedPreloadData(t, db)
			bob := testPerson{Name: "bob"}
			if err := driver.Create(db, &bob); err != nil {
				t.Fatal(err)
			}
			addEdge(t, db, person.ID, "best_friend", bob.ID)

			result, err := driver.Model[testPerson](db).
				Where("name", comparator.EQ, "alice").
				Preload("Topics", "BestFriend").
				Take()
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Topics) != 2 {
				t.Errorf("Expected 2 topics, got %d", len(result.Topics))
			}
			if result.BestFriend == nil || result.BestFriend.Name != "bob" {
				t.Errorf("Expected best friend bob, got %+v", result.BestFriend)
			}
		},
	)

	t.Run(
		"TestPreloadWithID", func(t *testing.T) {
			t.Cleanup(cleanDB)
			person, _ := seedPreloadData(t, db)

			result, err := driver.Model[testPerson](db).Preload("Topics").ID(person.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Topics) != 2 {
				t.Errorf("Expected 2 topics, got %d", len(result.Topics))
			}
		},
	)

	t.Run(
		"TestPreloadUnknownFieldErrors", func(t *testing.T) {
			t.Cleanup(cleanDB)
			seedPreloadData(t, db)

			_, err := driver.Model[testPerson](db).Preload("NotAField").Find()
			if err == nil {
				t.Error("Expected error for unknown preload field")
			}
			_, err = driver.Model[testPerson](db).Preload("NotAField").Take()
			if err == nil {
				t.Error("Expected error for unknown preload field on Take")
			}
		},
	)

	t.Run(
		"TestPreloadFieldWithoutEdgeTagErrors", func(t *testing.T) {
			t.Cleanup(cleanDB)
			seedPreloadData(t, db)

			_, err := driver.Model[testPerson](db).Preload("Name").Find()
			if err == nil {
				t.Error("Expected error for preload field without gremlinEdge tag")
			}
		},
	)

	t.Run(
		"TestEdgeFieldsNotPersistedAsProperties", func(t *testing.T) {
			person := testPerson{
				Name:   "alice",
				Topics: []testTopic{{Title: "graphs"}},
			}
			mapValue, err := driver.StructToMapForTest(&person)
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"Topics", "BestFriend", "Friends", "topics"} {
				if _, ok := mapValue[key]; ok {
					t.Errorf("Expected edge field %s to be excluded from properties", key)
				}
			}
		},
	)
}

func TestParseGremlinEdgeTag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		tag           string
		wantLabel     string
		wantDirection int
		wantErr       bool
	}{
		{name: "label only defaults to out", tag: "subscribed", wantLabel: "subscribed", wantDirection: 0},
		{name: "explicit out", tag: "subscribed,out", wantLabel: "subscribed", wantDirection: 0},
		{name: "in direction", tag: "subscribed,in", wantLabel: "subscribed", wantDirection: 1},
		{name: "both direction", tag: "friend,both", wantLabel: "friend", wantDirection: 2},
		{name: "empty tag errors", tag: "", wantErr: true},
		{name: "dash label errors", tag: "-", wantErr: true},
		{name: "unknown option errors", tag: "subscribed,sideways", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				opts, err := driver.ParseGremlinEdgeTagForTest(tt.tag)
				if tt.wantErr {
					if err == nil {
						t.Error("Expected error, got nil")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if opts.Label != tt.wantLabel {
					t.Errorf("Expected label %s, got %s", tt.wantLabel, opts.Label)
				}
				if opts.Direction != tt.wantDirection {
					t.Errorf("Expected direction %d, got %d", tt.wantDirection, opts.Direction)
				}
			},
		)
	}
}
