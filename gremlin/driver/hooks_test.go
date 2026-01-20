package driver

import (
	"fmt"
	"testing"

	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

type hookCreateVertex struct {
	gsmtypes.Vertex
	Name     string `json:"name" gremlin:"name"`
	HookNote string `json:"hook_note" gremlin:"hook_note"`

	beforeCreateCalled bool
	afterCreateCalled  bool
	beforeDriver       *GremlinDriver
	afterDriver        *GremlinDriver
	afterHadID         bool
	afterHadModifiedAt bool
}

func (v *hookCreateVertex) BeforeCreate(db *GremlinDriver) error {
	v.beforeCreateCalled = true
	v.beforeDriver = db
	v.HookNote = "before-create"
	return nil
}

func (v *hookCreateVertex) AfterCreate(db *GremlinDriver) error {
	v.afterCreateCalled = true
	v.afterDriver = db
	v.afterHadID = v.ID != nil
	v.afterHadModifiedAt = !v.LastModified.IsZero()
	return nil
}

type hookUpdateVertex struct {
	gsmtypes.Vertex
	Name     string `json:"name" gremlin:"name"`
	HookNote string `json:"hook_note" gremlin:"hook_note"`

	beforeUpdateCalled bool
	afterUpdateCalled  bool
	beforeDriver       *GremlinDriver
	afterDriver        *GremlinDriver
	afterHadID         bool
	afterHadModifiedAt bool
}

func (v *hookUpdateVertex) BeforeUpdate(db *GremlinDriver) error {
	v.beforeUpdateCalled = true
	v.beforeDriver = db
	v.HookNote = "before-update"
	return nil
}

func (v *hookUpdateVertex) AfterUpdate(db *GremlinDriver) error {
	v.afterUpdateCalled = true
	v.afterDriver = db
	v.afterHadID = v.ID != nil
	v.afterHadModifiedAt = !v.LastModified.IsZero()
	return nil
}

type hookFindVertex struct {
	gsmtypes.Vertex
	Name     string `json:"name" gremlin:"name"`
	HookNote string `json:"hook_note" gremlin:"hook_note"`

	afterFindCalled bool
	afterDriver     *GremlinDriver
}

func (v *hookFindVertex) AfterFind(db *GremlinDriver) error {
	v.afterFindCalled = true
	v.afterDriver = db
	v.HookNote = "after-find"
	return nil
}

type hookFindErrorVertex struct {
	gsmtypes.Vertex
	Name string `json:"name" gremlin:"name"`
}

func (v *hookFindErrorVertex) AfterFind(db *GremlinDriver) error {
	return fmt.Errorf("after find failed")
}

func TestCreateHooksCreate(t *testing.T) {
	db, err := Open(DbURL, dbDriver)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Cleanup(cleanDB)

	vertex := &hookCreateVertex{Name: "hook-test"}
	if err := Create(db, vertex); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if !vertex.beforeCreateCalled || !vertex.afterCreateCalled {
		t.Fatalf(
			"expected hooks to be called, before=%v after=%v",
			vertex.beforeCreateCalled, vertex.afterCreateCalled,
		)
	}
	if vertex.beforeDriver != db || vertex.afterDriver != db {
		t.Error("expected hook to receive the same driver instance")
	}
	if !vertex.afterHadID || !vertex.afterHadModifiedAt {
		t.Error("expected AfterCreate to see populated timestamps and ID")
	}

	loaded, err := Model[hookCreateVertex](db).ID(vertex.ID)
	if err != nil {
		t.Fatalf("ID() error = %v", err)
	}
	if loaded.HookNote != "before-create" {
		t.Errorf("expected hook note to be persisted, got %s", loaded.HookNote)
	}
}

func TestUpdateHooksUpdate(t *testing.T) {
	db, err := Open(DbURL, dbDriver)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Cleanup(cleanDB)

	vertex := &hookUpdateVertex{Name: "hook-test"}
	if err := Create(db, vertex); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	vertex.beforeUpdateCalled = false
	vertex.afterUpdateCalled = false
	vertex.HookNote = ""
	vertex.Name = "hook-test-updated"

	if err := Save(db, vertex); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if !vertex.beforeUpdateCalled || !vertex.afterUpdateCalled {
		t.Fatalf(
			"expected hooks to be called, before=%v after=%v",
			vertex.beforeUpdateCalled, vertex.afterUpdateCalled,
		)
	}
	if !vertex.afterHadID || !vertex.afterHadModifiedAt {
		t.Error("expected AfterUpdate to see populated timestamps and ID")
	}

	loaded, err := Model[hookUpdateVertex](db).ID(vertex.ID)
	if err != nil {
		t.Fatalf("ID() error = %v", err)
	}
	if loaded.HookNote != "before-update" {
		t.Errorf("expected hook note to be persisted, got %s", loaded.HookNote)
	}
	if loaded.Name != "hook-test-updated" {
		t.Errorf("expected updated name, got %s", loaded.Name)
	}
}

func TestAfterFindHooks(t *testing.T) {
	db, err := Open(DbURL, dbDriver)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Cleanup(cleanDB)

	vertices := []hookFindVertex{
		{Name: "hook-find-1"},
		{Name: "hook-find-2"},
	}
	for i := range vertices {
		if err := Create(db, &vertices[i]); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	results, err := Model[hookFindVertex](db).Find()
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if len(results) != len(vertices) {
		t.Fatalf("expected %d results, got %d", len(vertices), len(results))
	}
	for _, result := range results {
		if !result.afterFindCalled {
			t.Error("expected AfterFind to be called for Find() result")
		}
		if result.afterDriver != db {
			t.Error("expected AfterFind to receive the same driver instance")
		}
		if result.HookNote != "after-find" {
			t.Errorf("expected hook note to be set, got %s", result.HookNote)
		}
	}

	taken, err := Model[hookFindVertex](db).Take()
	if err != nil {
		t.Fatalf("Take() error = %v", err)
	}
	if !taken.afterFindCalled || taken.HookNote != "after-find" {
		t.Error("expected AfterFind to run for Take()")
	}

	found, err := Model[hookFindVertex](db).ID(vertices[0].ID)
	if err != nil {
		t.Fatalf("ID() error = %v", err)
	}
	if !found.afterFindCalled || found.HookNote != "after-find" {
		t.Error("expected AfterFind to run for ID()")
	}
}

func TestAfterFindHookError(t *testing.T) {
	db, err := Open(DbURL, dbDriver)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Cleanup(cleanDB)

	vertex := hookFindErrorVertex{Name: "hook-find-error"}
	if err := Create(db, &vertex); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = Model[hookFindErrorVertex](db).Take()
	if err == nil {
		t.Fatal("expected AfterFind error to be returned")
	}
}
