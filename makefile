# Makefile - GraphStructManager development tasks
MODULE := $(shell head -1 go.mod | awk '{print $$2}')
COVERAGE_THRESHOLD := 80
PATCH_THRESHOLD := 80
MUTATION_THRESHOLD := 60

.PHONY: lint first-setup go-lint test-cov test test-verbose coverage coverage-check patch-coverage vet security mutation deadcode build-check bench profile verify clean

## Lint via pre-commit hooks (golangci-lint, formatting)
lint:
	pre-commit run --all-files

## One-time machine setup (macOS/Homebrew)
first-setup:
	brew install golangci/tap/golangci-lint
	brew upgrade golangci-lint
	brew install pre-commit
	brew install go
	pre-commit install

## Autofix lint + format issues
go-lint:
	golangci-lint fmt ./...
	golangci-lint run --fix --allow-parallel-runners ./...

## Coverage percentage per package (quick, no gate)
test-cov:
	go test -cover ./...

## Canonical test run: race detector + shuffled order (used by verify)
test:
	go test -race -shuffle=on -count=1 ./...

## Verbose race test run
test-verbose:
	go test -race -shuffle=on -count=1 -v ./...

## Coverage report + open HTML view (no gate)
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out

## Coverage with threshold gate: fails under $(COVERAGE_THRESHOLD)%
coverage-check:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Coverage: $${COVERAGE}%"; \
	if [ $$(echo "$${COVERAGE} < $(COVERAGE_THRESHOLD)" | bc -l) -eq 1 ]; then \
		echo "FAIL: Coverage $${COVERAGE}% is below threshold $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	fi

## Coverage of lines changed vs main, mapped against real profile line ranges.
## Non-executable changed lines (comments, blanks, decls) are ignored.
## Fails under $(PATCH_THRESHOLD)% when commits are ahead of main with Go changes.
patch-coverage:
	@MERGE_BASE=$$(git merge-base main HEAD 2>/dev/null || echo ""); \
	if [ -z "$$MERGE_BASE" ]; then \
		echo "SKIP: main branch not available"; exit 0; \
	fi; \
	if [ "$$MERGE_BASE" = "$$(git rev-parse HEAD)" ]; then \
		echo "SKIP: no commits ahead of main"; exit 0; \
	fi; \
	CHANGED_FILES=$$(git diff --name-only $$MERGE_BASE HEAD -- '*.go' | grep -v '_test\.go' || true); \
	if [ -z "$$CHANGED_FILES" ]; then \
		echo "SKIP: no non-test Go files changed"; exit 0; \
	fi; \
	echo "Changed non-test Go files vs main:"; echo "$$CHANGED_FILES" | sed 's/^/  /'; \
	go test -coverprofile=patch_coverage.out -covermode=atomic ./... > /dev/null 2>&1 || true; \
	if [ ! -f patch_coverage.out ]; then \
		echo "FAIL: no coverage profile produced"; exit 1; \
	fi; \
	git diff --unified=0 $$MERGE_BASE HEAD -- $$CHANGED_FILES | awk ' \
		/^\+\+\+ b\// {f=substr($$0,7)} \
		/^@@/ {split($$3,a,","); ln=substr(a[1],2)+0; n=a[2]+0; for (i=0; i<n; i++) print f ":" ln+i} \
	' > patch_lines.tmp; \
	awk -v mod="$(MODULE)" -v thr="$(PATCH_THRESHOLD)" ' \
		FNR==NR {split($$0,p,":"); f=p[1]; nf[f]++; cl[f,nf[f]]=p[2]+0; next} \
		/^mode:/ {next} \
		{split($$0,c," "); split(c[1],fp,":"); path=fp[1]; rng=fp[2]; ct=c[3]+0; \
		 rel=path; \
		 if (index(rel, mod "/") == 1) rel=substr(rel, length(mod) + 2); \
		 ic[rel]++; split(rng, ab, ","); split(ab[1], s, "."); split(ab[2], e, "."); \
		 ist[rel,ic[rel]]=s[1]+0; ien[rel,ic[rel]]=e[1]+0; icn[rel,ic[rel]]=ct} \
		END {total=0; cov=0; unc=""; \
		 for (f in nf) for (i=1; i<=nf[f]; i++) {ln=cl[f,i]; state=0; \
		   for (j=1; j<=ic[f]; j++) if (ln >= ist[f,j] && ln <= ien[f,j]) {if (icn[f,j] > 0) {state=1; break} state=-1} \
		   if (state==0) continue; total++; \
		   if (state==1) cov++; else unc=unc sprintf("  %s:%d\n", f, ln)} \
		 if (total==0) {print "No executable changed lines detected"; exit 0} \
		 pct=int(cov*100/total); \
		 print "Patch coverage: " cov "/" total " lines = " pct "%"; \
		 if (unc != "") {print "Uncovered changed lines:"; printf "%s", unc} \
		 if (pct < thr+0) {print "FAIL: patch coverage " pct "% is below threshold " thr "%"; exit 1} \
		 print "PASS: patch coverage " pct "% meets threshold " thr "%"}' patch_lines.tmp patch_coverage.out

## go vet only (no autofix, no pre-commit)
vet:
	go vet ./...

## Security scanning
security:
	gosec ./...
	govulncheck ./...

## Mutation testing via gremlins (gate: efficacy >= $(MUTATION_THRESHOLD)%; not part of verify)
mutation:
	gremlins unleash --workers 1 --timeout-coefficient 3 --threshold-efficacy $(MUTATION_THRESHOLD)

## Unreachable functions (tests as entrypoints — correct mode for a library whose
## callers live outside this module; plain mode false-flags the entire public API)
deadcode:
	deadcode -test ./...

## Build validation
build-check:
	go build ./...
	go mod verify

## Benchmarks with allocation counts
bench:
	go test -bench=. -benchmem -count=3 -run=^$$ ./... | tee bench.txt

## CPU + memory profiles for pprof
profile:
	go test -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof -run=^$$ ./...
	@echo "CPU profile: go tool pprof cpu.prof"
	@echo "Memory profile: go tool pprof mem.prof"

## Full verification suite
verify: lint test coverage-check patch-coverage security deadcode build-check
	@echo "All verification checks passed."

clean:
	rm -f coverage.out patch_coverage.out patch_lines.tmp bench.txt cpu.prof mem.prof
	rm -rf dist/
