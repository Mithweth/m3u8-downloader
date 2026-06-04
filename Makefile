GO ?= go
REMOVE ?= rm
INSTALLBIN ?= install

ifeq ($(PREFIX),)
    PREFIX := /usr/local
endif

default: build

build: fmt
	@$(GO) mod download
	@$(GO) build -o m3u8-downloader cmd/cli/main.go

fmt:
	@$(GO) fmt ./...

vet:
	@$(GO) vet ./...

test: fmt vet
	@$(GO) test ./... -coverprofile=cover.out

coverage:
	@$(GO) tool cover -func=cover.out

clean:
	@$(REMOVE) -f m3u8-downloader cover.out

install: 
	$(INSTALLBIN) -d $(PREFIX)/bin/
	$(INSTALLBIN) -m 755 m3u8-downloader $(PREFIX)/bin/

uninstall:
	$(REMOVE) -f $(PREFIX)/bin/m3u8-downloader
