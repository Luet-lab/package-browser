UBINDIR ?= /usr/bin
SHAREDIR ?= /usr/share
DESTDIR ?=
GOOS?=linux
GOARCH?=amd64

all: build install

.PHONY: build
build:
	CGO_ENABLED=0 go build -o luet-package-browser main.go

install: build
	install -d $(DESTDIR)/$(UBINDIR)
	install -m 0755 luet-package-browser $(DESTDIR)/$(UBINDIR)/
	install -d $(DESTDIR)/$(SHAREDIR)/luet-package-browser
	install -m 0755 templates/index.tmpl $(DESTDIR)/$(SHAREDIR)/luet-package-browser
	install -m 0755 templates/package.tmpl $(DESTDIR)/$(SHAREDIR)/luet-package-browser
	install -m 0755 templates/packages.tmpl $(DESTDIR)/$(SHAREDIR)/luet-package-browser
	install -m 0755 templates/repository.tmpl $(DESTDIR)/$(SHAREDIR)/luet-package-browser
