.PHONY: build install clean lint test

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
