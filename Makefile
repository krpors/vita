.PHONY: all
HASH = $(shell git rev-parse HEAD)
all:
	go build -ldflags="-X main.CommitHash=${HASH}"

.PHONY: coverage
coverage:
	go test -v -coverprofile coverage.out .
	go tool cover -html coverage.out -o coverage.html
