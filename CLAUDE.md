## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).

Setup (fresh clone): install the CLI with `uv tool install graphifyy`, then bootstrap with `graphify update .`. The graph is deliberately not committed — derived output, ~5.6MB, and a merge-conflict magnet across branches.

Worktrees: do not regenerate per worktree. Link the canonical graph from the main checkout instead — `make graph-link` (or: `MAIN=$(git worktree list --porcelain | awk '/^worktree /{print substr($0,10); exit}') && ln -sfn "$MAIN/graphify-out" graphify-out`). `graphify update` run from any worktree refreshes the shared copy, so it stays current everywhere.

## Go AI-Verified Development

### Project Architecture

- `cmd/` — application entrypoint (`cmd/app.go`)
- `gsmtypes/` — shared types, interfaces, constants, errors (public API surface)
- `gremlin/driver/` — Gremlin graph-DB driver: schema, queries, edges, transactions, preload
- `comparator/` — graph struct comparison logic (currently untested)
- `log/` — structured logging with color handler
- Tests are colocated with sources: `foo.go` → `foo_test.go`

### Verification (required before every commit)

```
make verify   # lint + test + coverage gate + patch-coverage + security + deadcode + build-check
```

Individual checks:
```
make lint            # pre-commit hooks (golangci-lint, formatters)
make test            # go test -race -shuffle=on -count=1 ./...
make coverage-check  # gate: total coverage >= 80%
make patch-coverage  # gate: coverage of changed lines vs main >= 80%
make vet             # go vet only
make security        # gosec + govulncheck
make mutation        # gremlins, gate: efficacy >= 60% (slow; not part of verify)
make deadcode        # unreachable functions (tests as entrypoints; public API documented in README is not dead)
make build-check     # go build + go mod verify
make bench / profile # performance diagnostics (not part of verify)
```

### Quality Gates (from Makefile and .golangci.yml)

- Test coverage: >= 80% total; patch coverage >= 80% of changed executable lines
- Mutation score: >= 60%
- Complexity: cyclop <= 30 (package avg 10), gocognit <= 21, funlen <= 100 lines / 50 statements
- Formatting: gofumpt + gci + golines via `golangci-lint fmt` (run `make go-lint` to autofix)

### AI-Specific Rules

1. **No tautological tests**: encode expected outputs, don't reimplement the logic under test
2. **No hallucinated imports**: verify every dependency exists before adding it
3. **No lint suppression without permission**: never add `//nolint` or similar directives; fix the code, or ask before suppressing
4. **No vaporware**: every new package/function must be wired into non-test code
5. **Error wrapping**: `fmt.Errorf("context: %w", err)` on every error boundary
6. **Interfaces at the consumer, structs at the return**; `context.Context` first parameter, never stored in structs
7. **Explain non-obvious decisions**: comment WHY, not WHAT
8. **Integration tests for multi-component features**: wire real objects through the actual call chain, not mocks

### Git Policy

- Match existing history style: short imperative summary, PR number in the summary line, e.g. `Fix pointer handling on load and omitempty evaluation (#48)`
- One logical change per commit; no force-pushes to shared branches
