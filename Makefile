.PHONY: all
HASH = $(shell git rev-parse HEAD)
all:
	go build -ldflags="-X main.CommitHash=${HASH}"
