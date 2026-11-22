VERSION ?= 0.1.0
GO ?= go
BIN := bin/anygame
LDFLAGS := -s -w -X anygame/internal/cli.Version=$(VERSION)

.PHONY: all build check fmt vet test dist clean smoke

all: build

build:
	@mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/anygame

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

test:
	@mkdir -p .work-tmp/tmp
	TMPDIR=$(CURDIR)/.work-tmp/tmp $(GO) test ./...

check: fmt vet test

dist:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/anygame-linux-amd64 ./cmd/anygame
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/anygame-windows-amd64.exe ./cmd/anygame
	@ls -lh dist

clean:
	rm -rf bin dist

smoke:
	./scripts/smoke.sh
