package driver

import (
	"errors"
	"fmt"
)

var (
	// ErrNestedTransaction is returned when attempting to begin a transaction
	// from a driver that is already bound to an open transaction.
	ErrNestedTransaction = errors.New("nested transactions are not supported")
	// ErrNotInTransaction is returned when Commit or Rollback is called on a
	// driver that was not created by Begin or Transaction.
	ErrNotInTransaction = errors.New("driver is not in a transaction")
)

// Transaction executes fn inside a transaction. If fn returns an error or
// panics, the transaction is rolled back; otherwise it is committed.
//
// The *GremlinDriver passed to fn is bound to the transaction and works with
// all generic package-level functions:
//
//	err := db.Transaction(func(tx *driver.GremlinDriver) error {
//		if err := driver.Create(tx, &user); err != nil {
//			return err // rolls back
//		}
//		return driver.Model[Account](tx).
//			Where("owner", comparator.EQ, user.Name).
//			Update("balance", 100) // commit happens on nil return
//	})
//
// Note: transaction support depends on the backing graph database. The default
// TinkerGraph does not support transactions; Gremlin Server backed by a
// transaction-capable graph (e.g. JanusGraph, Neptune, TinkerTransactionGraph)
// is required.
func (driver *GremlinDriver) Transaction(fn func(tx *GremlinDriver) error) error {
	tx, err := driver.Begin()
	if err != nil {
		return err
	}

	panicked := true
	defer func() {
		if (panicked || err != nil) && tx.tx.IsOpen() {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				driver.logger.Errorf("failed to rollback transaction: %v", rollbackErr)
			}
		}
	}()

	err = fn(tx)
	if err == nil {
		err = tx.Commit()
	}
	panicked = false
	return err
}

// Begin starts a transaction and returns a driver bound to it. The returned
// driver must be finished with Commit or Rollback. Prefer Transaction for
// automatic commit/rollback handling.
func (driver *GremlinDriver) Begin() (*GremlinDriver, error) {
	if driver.tx != nil {
		return nil, ErrNestedTransaction
	}
	tx := driver.g.Tx()
	gtx, err := tx.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &GremlinDriver{
		remoteConn:  driver.remoteConn,
		g:           gtx,
		logger:      driver.logger,
		dbDriver:    driver.dbDriver,
		idGenerator: driver.idGenerator,
		tx:          tx,
	}, nil
}

// Commit commits the transaction this driver is bound to.
func (driver *GremlinDriver) Commit() error {
	if driver.tx == nil {
		return ErrNotInTransaction
	}
	if err := driver.tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Rollback rolls back the transaction this driver is bound to.
func (driver *GremlinDriver) Rollback() error {
	if driver.tx == nil {
		return ErrNotInTransaction
	}
	if err := driver.tx.Rollback(); err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	return nil
}

// InTransaction reports whether this driver is bound to a transaction.
func (driver *GremlinDriver) InTransaction() bool {
	return driver.tx != nil
}
