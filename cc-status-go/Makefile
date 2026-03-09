.PHONY: all build test clean install-hook install

all: build

build: bin/cc-status-tray bin/cc-status-hook

bin/cc-status-tray: $(shell find . -name '*.go' -not -path './assets/gen_icons.go')
	@mkdir -p bin
	go build -o $@ ./cmd/cc-status-tray

bin/cc-status-hook: $(shell find . -name '*.go' -not -path './assets/gen_icons.go')
	@mkdir -p bin
	go build -o $@ ./cmd/cc-status-hook

test:
	go test ./pkg/... ./internal/... -v

clean:
	rm -rf bin/

install-hook: bin/cc-status-hook
	./bin/cc-status-hook install

install: build install-hook
	mkdir -p ~/.local/bin
	cp bin/cc-status-tray ~/.local/bin/
	cp bin/cc-status-hook ~/.local/bin/
	@echo "Installed to ~/.local/bin/"
