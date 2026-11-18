# NetDiag — build, test and packaging helpers.

VERSION   ?= 1.4.0
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILD_DATE:= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)
GO        ?= go
PREFIX    ?= /usr/local
DESTDIR   ?=

.PHONY: all build build-pcap test test-race vet fmt fmt-check tidy run clean install release help

all: build

## build: binario portable (sin cgo, ARP pasivo)
build:
	@mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/netdiag ./cmd/netdiag
	@echo "→ bin/netdiag"

## build-pcap: binario con ARP activo (requiere libpcap-dev y cgo)
build-pcap:
	@mkdir -p bin
	CGO_ENABLED=1 $(GO) build -tags pcap -trimpath -ldflags '$(LDFLAGS)' -o bin/netdiag-pcap ./cmd/netdiag
	@echo "→ bin/netdiag-pcap"

## test: suite de pruebas
test:
	$(GO) test ./...

## test-race: pruebas con detector de carreras
test-race:
	$(GO) test -race ./...

## vet: análisis estático
vet:
	$(GO) vet ./...

## fmt: formatea el código
fmt:
	gofmt -w cmd internal

## fmt-check: falla si hay código sin formatear
fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || (echo "código sin formatear:"; gofmt -l cmd internal; exit 1)

## tidy: ordena go.mod/go.sum
tidy:
	$(GO) mod tidy

## run: inicia el servidor en primer plano
run:
	$(GO) run ./cmd/netdiag serve

## install: instala el binario en PREFIX/bin
install: build
	install -Dm755 bin/netdiag $(DESTDIR)$(PREFIX)/bin/netdiag
	@echo "instalado en $(DESTDIR)$(PREFIX)/bin/netdiag"

## release: binarios portables para Linux amd64 y arm64
release:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o dist/netdiag-linux-amd64 ./cmd/netdiag
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o dist/netdiag-linux-arm64 ./cmd/netdiag
	@echo "→ dist/"

## clean: elimina artefactos
clean:
	rm -rf bin dist

## help: muestra esta ayuda
help:
	@awk 'BEGIN {FS = ":.*## "} /^## / {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
