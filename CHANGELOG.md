# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.6.1] - 2026-10-12

### Fixed

- Pointer-element slices (`[]*string`, `[]*int64`, …) load correctly now: elements convert
  to the base type and allocate a pointer (`["a", null]` → `[*"a", nil]`), nil elements keep
  their nil pointer, and unconvertible elements keep the zero value. Previously such fields
  loaded as all nils. (#48)
- Lists shorter than a fixed-size array field no longer panic on load; the field is left
  untouched instead. (#48)
- `Query.Update` / `Query.Updates` handle pointer values the same way `Save` does:
  the value behind the pointer is written, not the pointer itself, so a plain value for a
  pointer-declared field (and a pointer value for a plain field) both work. A `*[]string`
  field is now dropped and rewritten per-element with list cardinality (set cardinality on
  Neptune) instead of being written as one single-cardinality property holding the whole
  list. Updating a pointer-to-slice with a nil pointer (`(*[]string)(nil)`) drops the
  existing property, like updating a plain nil slice; a nil pointer to a scalar
  (`(*string)(nil)`) writes a single null value. (#49)

### Changed

- `omitempty` follows the field's own zero value for pointer fields: a non-nil pointer to a
  zero value (e.g. `&false`, `&""`) is now persisted on create/update where it was previously
  omitted. Nil pointers are still omitted. Graph consumers should expect additional property
  writes for such fields. (#48)
