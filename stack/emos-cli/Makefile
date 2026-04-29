VERSION ?= 0.6.1
BINARY  := emos
GO      ?= $(shell which go 2>/dev/null || echo /usr/local/go/bin/go)
NPM     ?= $(shell which npm 2>/dev/null || echo /usr/bin/npm)
LDFLAGS := -ldflags "-s -w -X github.com/automatika-robotics/emos-cli/internal/config.Version=$(VERSION)"

WEB_DIR    := web
WEB_DIST   := internal/webui/dist
WEB_OUT    := $(WEB_DIST)/index.html
WEB_STAMP  := $(WEB_DIR)/node_modules/.package-lock.json

.PHONY: build build-all install clean tidy web web-dev web-clean check

# `make build` always rebuilds the SPA so the embedded UI is fresh.
build: web
	$(GO) build $(LDFLAGS) -o $(BINARY) .

build-all: web clean
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 .

install:
	sudo cp $(BINARY) /usr/local/bin/$(BINARY)
	@echo "Installed to /usr/local/bin/$(BINARY)"

# --- Frontend ---

# Vite drops output directly into internal/webui/dist via vite.config.ts.
# We only rebuild when source files are newer than the built index.html.
web: $(WEB_OUT)

# Run `npm install` whenever package-lock.json is newer than the install
# stamp under node_modules. Catches the "git pull added a new dep" case
# that a bare existence check misses (npm leaves a copy of package-lock
# inside node_modules after install, which is a reliable freshness mark).
$(WEB_STAMP): $(WEB_DIR)/package.json $(WEB_DIR)/package-lock.json
	@echo ">> installing web deps"
	cd $(WEB_DIR) && $(NPM) install --silent
	@touch $@

$(WEB_OUT): $(WEB_STAMP) $(shell find $(WEB_DIR)/src $(WEB_DIR)/index.html $(WEB_DIR)/vite.config.ts -type f 2>/dev/null)
	@echo ">> building web (vite)"
	# Clean only the build outputs — preserve placeholder.html (tracked) so a
	# subsequent fresh checkout keeps the embed target valid.
	rm -rf $(WEB_DIST)/assets $(WEB_DIST)/index.html
	cd $(WEB_DIR) && $(NPM) run build --silent

web-dev:
	cd $(WEB_DIR) && $(NPM) run dev

web-clean:
	rm -rf $(WEB_DIST)/* $(WEB_DIR)/node_modules

# --- Misc ---

tidy:
	$(GO) mod tidy

check:
	$(GO) vet ./...
	cd $(WEB_DIR) && $(NPM) run check

clean:
	rm -f $(BINARY)
	rm -rf dist/
