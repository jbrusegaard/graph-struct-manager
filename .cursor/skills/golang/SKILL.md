---
name: golang
description: GraphStructManager Go 1.27 + Gremlin guidelines
---

# Overview

You are maintaining GraphStructManager, a type-safe, chainable Gremlin query builder for Go.
Prioritize correctness, predictable API behavior, and clear mapping between Go structs and
Gremlin traversals.

## Inspiration

- The API is inspired by GORM, but adapted for graph databases and Gremlin traversals rather
  than SQL.

## Repository Context

- Go version: 1.27 (module `github.com/jbrusegaard/graph-struct-manager`)
- Gremlin: Apache TinkerPop `gremlin-go` v3.8.1 (servers must be 3.8.1+; see README Requirements)
- Core packages: `gremlin/driver`, `gsmtypes`, `comparator`, `log`
- Public surface: generic query builder (`Query[T gsmtypes.VertexType]`) and helper APIs

## Graph Model Conventions

- Vertex structs must embed `gsmtypes.Vertex` anonymously; `Create`/`Update` validate this.
- Edge structs embed `gsmtypes.Edge` anonymously and implement `gsmtypes.EdgeType`. Create
  edges with `CreateEdge(db, &edge, from, to)` / `SaveEdge`; endpoints accept vertex structs
  or raw IDs. Prefer `Edge[E](db)` for edge queries (`Model[E]` still works via schema
  `isEdge` and traverses `g.E()`).
- Query a vertex's edges with `Edge[E].From(vertex)` / `.To(vertex)` (starts at
  `g.V(id).OutE()`/`InE()`, not a `g.E()` scan); edge-query-only, mutually exclusive with
  `PreQuery`.
- Edge properties are single-valued: never pass a Cardinality argument when writing edge
  properties (TinkerPop rejects it); `Preload` is vertex-only.
- Use `gremlin:"field_name"` tags for properties; `gremlin:"field_name,omitempty"` skips
  zero values and nil pointers during create/update.
- Use `gremlinSubTraversal:"alias"` for subtraversal projections; the alias must match
  `AddSubTraversal` usage.
- Use `gremlinEdge:"edge_label[,out|in|both]"` for related-vertex fields loaded via
  `Preload(goFieldName)`; nested paths use dots (`Preload("Topics.Posts")`). These
  fields are never persisted as properties. `PreloadDedup` dedups the loaded slice
  by vertex identity at the relationship at the end of the path.
- Label resolution: `Label() string` wins; empty label defaults to snake_case struct name.

## Query Builder Design

- Keep methods chainable and consistent with existing behavior (mutate query, return `*Query[T]`).
- Use `comparator` constants for predicates; map them to Gremlin `P`/`TextP` consistently.
- Do not hand-build traversal strings for debugging; the dump is translated from Gremlin bytecode,
  so new steps show up automatically.
- `Range` is ignored when `Offset` is set; maintain this behavior and warnings.
- `Take()` returns the first result; `ID()` performs fast ID lookup using `HasLabel`.
- Never stack two `By()` modulators on `valueMap()`/`propertyMap()` — 3.8 servers reject the
  traversal outright ("can only have one by modulator", status 87). The read path uses exactly
  one `By(unfoldSingleValueTraversal())`.
- Keep the map read path on `valueMap(true)` + a single `By`: `elementMap` collapses
  SET-cardinality properties to one value server-side (verified on 3.8.1) and would silently
  drop values. Any future switch must keep `cardinality_load_test.go` green deliberately.

## Struct Mapping and Tags

- `structToMap` drives create/update; it ignores `gremlinSubTraversal` tags.
- `UnloadGremlinResultIntoStruct` maps Gremlin results to tagged fields, including anonymous
  embedded structs and subtraversal aliases.
- When changing mapping rules, update tests under `gremlin/driver/*_test.go`.

## Logging and Diagnostics

- Logging uses `log.InitializeLogger()` and `GSM_LOG_LEVEL`.
- Query debugging (`GSM_DEBUG=true`, read when a driver opens) logs every executed traversal via
  `GremlinDriver.logTraversal` in `gremlin/driver/debug.go`, translated from its bytecode.
- Call `db.logTraversal(traversal)` immediately before executing a traversal, on the final traversal
  (after valueMap/preload/order steps are applied).

## Error Handling

- Return explicit errors; wrap with context (`fmt.Errorf("context: %w", err)`).
- Avoid panics and hidden behavior changes; align error messages with existing usage.

## Testing and Docs

- Prefer table-driven tests and cover exported behavior.
- Live tests hit `GREMLIN_TEST_URL` (default `ws://localhost:8182`, server must be 3.8.1+) in
  `GREMLIN_TEST_DRIVER` mode (`neptune` default = set cardinality; `gremlin` = list). Run both
  modes when touching cardinality or the map read path.
- If you change public API or query semantics, update `README.md` examples to match.
