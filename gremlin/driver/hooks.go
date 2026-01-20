package driver

import "fmt"

// BeforeUpdateHook runs before update persists the vertex.
// Returning an error aborts the update.
type BeforeUpdateHook interface {
	BeforeUpdate(db *GremlinDriver) error
}

// AfterUpdateHook runs after update persists the vertex.
// Returning an error propagates to the caller.
type AfterUpdateHook interface {
	AfterUpdate(db *GremlinDriver) error
}

// AfterFindHook runs after a vertex has been loaded from the database.
// Returning an error propagates to the caller.
type AfterFindHook interface {
	AfterFind(db *GremlinDriver) error
}

func runBeforeUpdateHook[T any](db *GremlinDriver, value *T) error {
	if value == nil {
		return nil
	}
	hook, ok := any(value).(BeforeUpdateHook)
	if !ok {
		return nil
	}
	if err := hook.BeforeUpdate(db); err != nil {
		return fmt.Errorf("before update hook: %w", err)
	}
	return nil
}

func runAfterUpdateHook[T any](db *GremlinDriver, value *T) error {
	if value == nil {
		return nil
	}
	hook, ok := any(value).(AfterUpdateHook)
	if !ok {
		return nil
	}
	if err := hook.AfterUpdate(db); err != nil {
		return fmt.Errorf("after update hook: %w", err)
	}
	return nil
}

func runAfterFindHook[T any](db *GremlinDriver, value *T) error {
	if value == nil {
		return nil
	}
	hook, ok := any(value).(AfterFindHook)
	if !ok {
		return nil
	}
	if err := hook.AfterFind(db); err != nil {
		return fmt.Errorf("after find hook: %w", err)
	}
	return nil
}

// BeforeCreateHook runs before create persists the vertex.
// Returning an error aborts the create.
type BeforeCreateHook interface {
	BeforeCreate(db *GremlinDriver) error
}

// AfterCreateHook runs after create persists the vertex.
// Returning an error propagates to the caller.
type AfterCreateHook interface {
	AfterCreate(db *GremlinDriver) error
}

func runBeforeCreateHook[T any](db *GremlinDriver, value *T) error {
	if value == nil {
		return nil
	}
	hook, ok := any(value).(BeforeCreateHook)
	if !ok {
		return nil
	}
	if err := hook.BeforeCreate(db); err != nil {
		return fmt.Errorf("before create hook: %w", err)
	}
	return nil
}

func runAfterCreateHook[T any](db *GremlinDriver, value *T) error {
	if value == nil {
		return nil
	}
	hook, ok := any(value).(AfterCreateHook)
	if !ok {
		return nil
	}
	if err := hook.AfterCreate(db); err != nil {
		return fmt.Errorf("after create hook: %w", err)
	}
	return nil
}
