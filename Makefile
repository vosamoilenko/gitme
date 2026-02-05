PREFIX ?= /usr/local
BINDIR = $(PREFIX)/bin
MANDIR = $(PREFIX)/share/man/man1

.PHONY: build install uninstall clean

build:
	go build -o gitme .

install: build
	install -d $(BINDIR)
	install -m 755 gitme $(BINDIR)/gitme
	install -d $(MANDIR)
	install -m 644 gitme.1 $(MANDIR)/gitme.1

uninstall:
	rm -f $(BINDIR)/gitme
	rm -f $(MANDIR)/gitme.1

clean:
	rm -f gitme
