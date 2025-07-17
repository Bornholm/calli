SHELL := /bin/bash

LDFLAGS ?= -w -s
GCFLAGS ?= -trimpath=$(PWD)
ASMFLAGS ?= -trimpath=$(PWD)

build:
	CGO_ENABLED=0 go build \
		-ldflags "$(LDFLAGS)" \
		-gcflags "$(GCFLAGS)" \
		-asmflags "$(ASMFLAGS)" \
		-o ./bin/server ./cmd

watch: tools/modd/bin/modd
	tools/modd/bin/modd

config.yml: build
	[ -f config.yml ] || bin/server -dump-config > config.yml

GORELEASER_ARGS ?= release --auto-snapshot --clean

goreleaser: tools/goreleaser/bin/goreleaser
	REPO_OWNER=$(shell whoami) tools/goreleaser/bin/goreleaser $(GORELEASER_ARGS)

tools/goreleaser/bin/goreleaser:
	mkdir -p tools/goreleaser/bin
	GOBIN=$(PWD)/tools/goreleaser/bin go install github.com/goreleaser/goreleaser/v2@latest

tools/modd/bin/modd:
	mkdir -p tools/modd/bin
	GOBIN=$(PWD)/tools/modd/bin go install github.com/cortesi/modd/cmd/modd@latest

-include misc/*/*.mk