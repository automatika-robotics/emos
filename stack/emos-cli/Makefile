VERSION ?= 0.4.0
BINARY  := emos
LDFLAGS := -ldflags "-s -w -X github.com/automatika-robotics/emos-cli/internal/config.Version=$(VERSION)"

.PHONY: build build-all install clean tidy

build:
	go build $(LDFLAGS) -o $(BINARY) .

build-all: clean
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 .

install: build
	sudo cp $(BINARY) /usr/local/bin/$(BINARY)
	@echo "Installed to /usr/local/bin/$(BINARY)"

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
	rm -rf dist/
