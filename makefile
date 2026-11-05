.PHONY: lint
lint:
	pre-commit run --all-files

.PHONY: first-setup
first-setup:
	brew install golangci/tap/golangci-lint
	brew upgrade golangci-lint
	brew install pre-commit
	brew install go
	pre-commit install

.PHONY: go-lint
go-lint: # golangci-lint version is pinned here; keep in sync with CI and pre-commit
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt ./...
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --fix --allow-parallel-runners ./...

.PHONY: test-cov
test-cov:
	go test -cover ./...

.PHONY: test
test:
	go test ./...

.PHONY: coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out
