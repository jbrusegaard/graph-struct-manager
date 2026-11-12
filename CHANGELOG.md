# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- The Gremlin client is pinned to Apache TinkerPop `gremlin-go` v3.8.1 and the dev server
  image moves to TinkerPop 3.8.1. This makes a server at 3.8.1 or later (AWS Neptune engine
  1.5.0.0 or later) a hard requirement: the 3.8.1 driver emits the `discard()` step on
  mutation iterations, which pre-3.8 servers reject. Servers from the 3.6/3.7 era are no
  longer guaranteed; see the compatibility matrix in the README.
- `time.Time` and `*time.Time` properties now serialize as GraphBinary `OffsetDateTime`
  instead of the legacy `Date` type (a 3.8.0 driver change). Reads accept both formats, so
  existing data keeps loading, but loaded times decode in a fixed-offset zone rather than
  the sender's location — compare with `Equal`/`UTC`, not `==`. Rollback caveat: a pre-3.8
  Go driver cannot read dates written after this version, so reverting a shared database
  once writes have landed means restoring from a snapshot.
- The map read path deliberately stays on `valueMap(true)` with a single unfolding `By`
  modulator. The `elementMap` migration was evaluated against a 3.8.1 server and rejected:
  `elementMap` collapses SET-cardinality properties to one value server-side, silently
  dropping the rest of every multi-valued property.

### Added

- Live integration tests honor `GREMLIN_TEST_URL` (server URL, default
  `ws://localhost:8182`) and `GREMLIN_TEST_DRIVER` (`neptune` by default, `gremlin` for
  list cardinality), so the suite can run against any compliant server in either
  cardinality mode.
- Regression tests pinning 3.8.1 driver semantics: `OffsetDateTime` round trips, `OrderBy`
  across mixed UTC offsets, date range predicates, strictly increasing `last_modified`,
  single-vs-multi cardinality load shapes with narrowing reports, and the split between
  real server errors and `ErrNotFound`.
- `RawQuery.Has` rejects traversal values up front with a "use Where instead" error; the
  `has(key, traversal)` form it would forward was removed in TinkerPop 3.8.

### Fixed

- `Count` no longer swallows a mid-stream server error as a count of `0`; the underlying
  failure is returned instead. Genuinely empty results still count as `0`.

## [v0.6.5] - 2026-11-11

### Fixed

- Disabling automatic last-modified tracking via `gsmtypes.LastModifiedPropertyType`
  now fully takes effect for edge models (and embedded `types.Vertex` models):
  `CreateEdge`/`SaveEdge` previously persisted the embedded `last_modified` tag's
  zero value (`0001-01-01T00:00:00Z`) on every save even with tracking off, so
  there was no way to stop the property being written. A zero, never-set
  last-modified field is now skipped on struct-based writes; explicitly set
  values are still persisted through the model's `gremlin` tags. Fields whose
  type implements `gsmtypes.SerializerType` are exempt from the skip: their
  zero value still goes through the type's own serialization contract. The skip
  also covers renamed tracking (`LastModifiedProperty` returning a custom
  name): models following the rename recipe with an overridden setter no
  longer persist the embedded zero timestamp under `last_modified` either. (#60)

## [v0.6.4] - 2026-11-07

### Changed

- Errors from Gremlin server calls and custom `SerializerType`/`DeserializerType` hooks
  are now wrapped with operation context (`fmt.Errorf("query find: %w", err)` and
  similar) instead of being returned bare, so a failure names the operation that
  produced it. `errors.Is` and `errors.As` still match through the chain; code that
  matched exact error strings or type-asserted the raw driver error directly (for
  example `err.(*gremlingo.GremlinError)`) should switch to `errors.As`. (#57)

### Added

- `make verify` runs the commit-readiness gate in one command: lint, race tests,
  total coverage of at least 80%, patch coverage of at least 80% against `main`,
  gosec + govulncheck, a dead-code check, and a build check. Existing targets are
  unchanged. (#56)
- Contributor linting now covers test files, which a golangci-lint v1-to-v2 migration
  had silently exempted, and the `wrapcheck`, `thelper`, `misspell` and `prealloc`
  linters are enabled; all 130 findings this surfaced are cleared, with no `//nolint`
  suppressions. (#57)

## [v0.6.3] - 2026-11-04

### Fixed

- An integer property no longer loads into a `string` field as a corrupted single
  character. Go treats integer-to-`string` as a convertible pair, but the conversion is
  a rune mapping, not a decimal rendering: `int32(42)` became `"*"` and `int64(2026)`
  became a stray code point. Integer-to-`string` now renders the decimal digits
  (`"42"`, `"2026"`), so a load followed by a save keeps the digits instead of writing
  back a corrupted or zero value. Floating-point sources are rendered the same way
  (`float64(3.5)` becomes `"3.5"`, `float64(8)` becomes `"8"`); previously a float
  arriving at a `string` field was skipped and the field loaded as its zero value.
  Note the limit: a field modelled as `string` holds
  text after a load, so saving writes a String property back to Neptune; a numeric
  predicate such as `Where("rank", comparator.EQ, 42)` will not match a vertex whose
  numeric property was modelled as `string`. Model large integers as `int64` to keep the
  property numeric through a round trip. The reverse direction is now guarded too: Go
  also converts `string` to `[]rune` (which is `[]int32`), so a string property loading
  into an `[]int32` field became character codes (`"80"` became `{56, 48}`); a `string`
  now converts only to `string` or `[]byte`. Numeric-to-numeric and `[]byte`-to-`string`
  conversions are unaffected, and `byte` (`uint8`) sources keep their character semantics:
  `byte('A')` loads into a `string` field as `"A"`, not `"65"`, so a `[]byte` arriving at
  a `[]string` field still loads as characters. (#55)

## [v0.6.2] - 2026-10-15

### Fixed

- A property holding several values no longer loads a single-valued field as its zero
  value. Every Neptune vertex property is set-cardinality, so a field that models one
  value still receives a list once the property holds more than one; the first value is
  now kept, matching how Gremlin itself narrows a property reference. Applies to every
  scalar kind (`string`, numeric, `bool`). Slice fields, `[]byte`-to-`string` conversions
  and fixed-size arrays are unaffected. (#51)

### Added

- `Find`, `Take` and `ID` log a warning naming the property, the struct field and how many
  values were discarded whenever the narrowing above happens. Set-cardinality writes are
  easy to make by accident and hard to discover from the loaded struct alone, so the loss
  is surfaced rather than left to be noticed as an unexpectedly blank or stale field. (#51)
- `UnloadGremlinResultIntoStructReportingNarrowed` returns the same information as
  `[]NarrowedProperty` for callers that unload results themselves.
  `UnloadGremlinResultIntoStruct` is unchanged. Narrowing inside a preloaded vertex is not
  reported, for the same reason its errors are not: that path cannot surface either. (#51)

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
