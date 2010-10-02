include $(GOROOT)/src/Make.inc

PKGDIR=$(GOROOT)/pkg/$(GOOS)_$(GOARCH)

TARG=unsafe/coffer
CGOFILES=coffer.go
CGO_CFLAGS=-I. -I "$(GOROOT)/include"
GOFMT=$(GOROOT)/bin/gofmt -tabwidth=4 -spaces=true -tabindent=false -w 

include $(GOROOT)/src/Make.pkg

CLEANFILES+=main $(PKGDIR)/$(TARG).a

again: clean install

format: 
	$(GOFMT) coffer.go
