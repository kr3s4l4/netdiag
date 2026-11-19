# NetDiag — build, test and packaging helpers.

VERSION   ?= 1.5.1
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILD_DATE:= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)
GO        ?= go
PREFIX    ?= /usr/local
DESTDIR   ?=

# F37: publicación reproducible. Estas variables se pasan desde el entorno;
# la clave de firma NUNCA se guarda en el repositorio.
RELEASE_VERSION ?= $(VERSION)
SIGNING_JSON    ?=
PUBLIC_KEY      ?=
NOTES_FILE      ?=
RELEASE_BASE_URL ?=
MIN_VERSION     ?=
RELEASE_OUT     ?=
RELEASE_DIR     ?=
KEYS_DIR        ?=
DELIVERY_RELEASE ?=
DELIVERY_LICENSE ?=
DELIVERY_OUT     ?=

.PHONY: all build build-pcap test test-race vet fmt fmt-check tidy run clean install release release-secure verify-release release-keygen delivery-zip help

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

## release-keygen: crea una clave de PRUEBA en KEYS_DIR (nunca para releases oficiales)
release-keygen:
	@test -n "$(KEYS_DIR)" || { echo "Define KEYS_DIR=/ruta/privada" >&2; exit 1; }
	$(GO) run ./cmd/netdiag-release keygen --out "$(KEYS_DIR)"

## release-secure: build reproducible, firmas, SBOM, procedencia y .ndc (F37)
release-secure:
	@test -n "$(SIGNING_JSON)" || { echo "Define SIGNING_JSON=/ruta/signing.json" >&2; exit 1; }
	@test -n "$(NOTES_FILE)" || { echo "Define NOTES_FILE=/ruta/RELEASE_NOTES.md" >&2; exit 1; }
	scripts/release.sh --version "$(RELEASE_VERSION)" \
		--signing-json "$(SIGNING_JSON)" \
		$(if $(PUBLIC_KEY),--public-key "$(PUBLIC_KEY)",) \
		--notes-file "$(NOTES_FILE)" \
		$(if $(RELEASE_BASE_URL),--base-url "$(RELEASE_BASE_URL)",) \
		$(if $(MIN_VERSION),--min-version "$(MIN_VERSION)",) \
		$(if $(RELEASE_OUT),--out "$(RELEASE_OUT)",)

## verify-release: verifica un release firmado sin red (F37)
verify-release:
	@test -n "$(RELEASE_DIR)" || { echo "Define RELEASE_DIR=/ruta/al/release" >&2; exit 1; }
	scripts/verify-release.sh --dir "$(RELEASE_DIR)" \
		$(if $(RELEASE_VERSION),--version "$(RELEASE_VERSION)",) \
		$(if $(PUBLIC_KEY),--key "$(PUBLIC_KEY)",)

## delivery-zip: empaqueta release firmado y licencia para entrega manual
delivery-zip:
	@test -n "$(DELIVERY_RELEASE)" || { echo "Define DELIVERY_RELEASE=/ruta/al/release" >&2; exit 1; }
	@test -n "$(DELIVERY_LICENSE)" || { echo "Define DELIVERY_LICENSE=/ruta/license.json" >&2; exit 1; }
	@test -n "$(DELIVERY_OUT)" || { echo "Define DELIVERY_OUT=/ruta/paquete.zip" >&2; exit 1; }
	bash scripts/pack-delivery.sh --release-dir "$(DELIVERY_RELEASE)" \
		--license "$(DELIVERY_LICENSE)" --out "$(DELIVERY_OUT)"

## clean: elimina artefactos
clean:
	rm -rf bin dist

## help: muestra esta ayuda
help:
	@awk 'BEGIN {FS = ":.*## "} /^## / {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
