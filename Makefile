.PHONY: build install clean lint test hooks

hooks:
	git config core.hooksPath .githooks
	@chmod +x .githooks/pre-commit

build:
	go build -o bin/td .

install:
	go install .

clean:
	rm -rf bin/

lint:
	golangci-lint run

test:
	go test ./...
