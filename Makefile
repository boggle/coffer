include $(GOROOT)/src/Make.inc

PKGDIR=$(GOROOT)/pkg/$(GOOS)_$(GOARCH)


TARG=unsafe/coffer
CGOFILES=\
	 coffer.go

LDPATH_freebsd=-Wl,-R,`pwd`
LDPATH_linux=-Wl,-R,`pwd`
LDPATH_darwin=

CGO_CFLAGS=-I. -I "$(GOROOT)/include"
CGO_LDFLAGS=_cgo_export.o coffer_cb.so $(LDPATH_$(GOOS))
CGO_DEPS=_cgo_export.o coffer_cb.so

GOFMT=$(GOROOT)/bin/gofmt -tabwidth=4 -spaces=true -tabindent=false -w 

CLEANFILES+=main $(PKGDIR)/$(TARG).a coffer_cb.o coffer_cb.so

include $(GOROOT)/src/Make.pkg

coffer_cb.o: coffer_cb.c _cgo_export.h
	gcc $(_CGO_CFLAGS_$(GOARCH)) -g -c -fPIC $(CFLAGS) coffer_cb.c

coffer_cb.so: coffer_cb.o
	gcc $(_CGO_CFLAGS_$(GORACH)) -o $@ coffer_cb.o $(_CGO_LDFLAGS_$(GOOS))

again: clean install

format: 
	$(GOFMT) coffer.go
