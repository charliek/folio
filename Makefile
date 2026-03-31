.PHONY: build test lint fmt tidy clean check coverage docs docs-serve docker-build

build:
	cd server && go build -o folio .

test:
	cd server && go test -v ./...

lint:
	cd server && golangci-lint run -c ../.golangci.yml

fmt:
	cd server && go fmt ./...

tidy:
	cd server && go mod tidy

clean:
	rm -f server/folio

check: lint test

coverage:
	cd server && go test -v -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: server/coverage.html"

docs:
	uv sync --group docs
	uv run mkdocs build --strict

docs-serve:
	uv sync --group docs
	uv run mkdocs serve

docker-build:
	docker build -t folio server/
