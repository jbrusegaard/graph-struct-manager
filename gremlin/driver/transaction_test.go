package driver_test

import (
	"errors"
	"testing"

	"github.com/jbrusegaard/graph-struct-manager/comparator"
	"github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
	"github.com/jbrusegaard/graph-struct-manager/gsmtypes"
)

func openTestDB(t *testing.T) *driver.GremlinDriver {
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

func cleanupVertexByName(t *testing.T, db *driver.GremlinDriver, name string) {
	t.Helper()
	t.Cleanup(
		func() {
			err := driver.Model[testVertex](db).
				Where("name", comparator.EQ, name).
				Delete()
			if err != nil {
				t.Logf("cleanup failed for %s: %v", name, err)
			}
		},
	)
}

func TestTransactionCommit(t *testing.T) {
	db := openTestDB(t)
	name := "tx-commit-test"
	cleanupVertexByName(t, db, name)

	testV := &testVertex{Name: name}
	err := db.Transaction(
		func(tx *driver.GremlinDriver) error {
			if !tx.InTransaction() {
				t.Error("tx driver should report InTransaction() == true")
			}
			return driver.Create(tx, testV)
		},
	)
	if err != nil {
		t.Fatalf("Transaction() should not error, got %v", err)
	}

	vertex, err := driver.Model[testVertex](db).ID(testV.ID)
	if err != nil {
		t.Fatalf("committed vertex should be findable, got %v", err)
	}
	if vertex.Name != name {
		t.Errorf("vertex name should be %s, got %s", name, vertex.Name)
	}
}

func TestTransactionRollbackOnError(t *testing.T) {
	db := openTestDB(t)
	name := "tx-rollback-test"
	cleanupVertexByName(t, db, name)

	errAbort := errors.New("abort transaction")
	testV := &testVertex{Name: name}
	err := db.Transaction(
		func(tx *driver.GremlinDriver) error {
			if createErr := driver.Create(tx, testV); createErr != nil {
				return createErr
			}
			return errAbort
		},
	)
	if !errors.Is(err, errAbort) {
		t.Fatalf("Transaction() should return fn error, got %v", err)
	}

	_, err = driver.Model[testVertex](db).ID(testV.ID)
	if !errors.Is(err, gsmtypes.ErrNotFound) {
		t.Errorf("rolled back vertex should not be findable, got %v", err)
	}
}

func TestTransactionRollbackOnPanic(t *testing.T) {
	db := openTestDB(t)
	name := "tx-panic-test"
	cleanupVertexByName(t, db, name)

	testV := &testVertex{Name: name}
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Transaction() should re-panic after rollback")
			}
		}()
		_ = db.Transaction(
			func(tx *driver.GremlinDriver) error {
				if createErr := driver.Create(tx, testV); createErr != nil {
					return createErr
				}
				panic("boom")
			},
		)
	}()

	_, err := driver.Model[testVertex](db).ID(testV.ID)
	if !errors.Is(err, gsmtypes.ErrNotFound) {
		t.Errorf("panicked transaction vertex should not be findable, got %v", err)
	}
}

func TestBeginCommit(t *testing.T) {
	db := openTestDB(t)
	name := "tx-begin-commit-test"
	cleanupVertexByName(t, db, name)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() should not error, got %v", err)
	}
	testV := &testVertex{Name: name}
	if err = driver.Create(tx, testV); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("Commit() should not error, got %v", err)
	}

	if _, err = driver.Model[testVertex](db).ID(testV.ID); err != nil {
		t.Errorf("committed vertex should be findable, got %v", err)
	}
}

func TestBeginRollback(t *testing.T) {
	db := openTestDB(t)
	name := "tx-begin-rollback-test"
	cleanupVertexByName(t, db, name)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() should not error, got %v", err)
	}
	testV := &testVertex{Name: name}
	if err = driver.Create(tx, testV); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatalf("Rollback() should not error, got %v", err)
	}

	_, err = driver.Model[testVertex](db).ID(testV.ID)
	if !errors.Is(err, gsmtypes.ErrNotFound) {
		t.Errorf("rolled back vertex should not be findable, got %v", err)
	}
}

func TestTransactionErrors(t *testing.T) {
	db := openTestDB(t)

	if err := db.Commit(); !errors.Is(err, driver.ErrNotInTransaction) {
		t.Errorf("Commit() on non-tx driver should return ErrNotInTransaction, got %v", err)
	}
	if err := db.Rollback(); !errors.Is(err, driver.ErrNotInTransaction) {
		t.Errorf("Rollback() on non-tx driver should return ErrNotInTransaction, got %v", err)
	}
	if db.InTransaction() {
		t.Error("non-tx driver should report InTransaction() == false")
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() should not error, got %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err = tx.Begin(); !errors.Is(err, driver.ErrNestedTransaction) {
		t.Errorf("nested Begin() should return ErrNestedTransaction, got %v", err)
	}
}
