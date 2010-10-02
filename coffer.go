// Package for copying between memory ranges managed by C code and Go Buffers
package coffer

import os "os"
import .  "gonewrong"
import "unsafe"
import "fmt"
import "io"

// #include <stdlib.h>
// #include <string.h>
import "C"

// A PtrCoffer implements the io.ReadWriteSeeker interface for
// a memory range
//
// This allows direct copying between a C memory range
// and a Go Buffer
type Coffer interface {
    io.ReadWriteSeeker
    io.Closer

	GetBasePtr() uintptr
	GetSeekPtr() uintptr
	GetStopPtr() uintptr
}

// Plain coffer on some memory range provided by the user
//
// This struct is not thread-safe
//
type PtrCoffer struct {
    base uintptr // base pointer
    seek uintptr // current pointer
    stop uintptr // pointer to last element
}

// Creates a new PtrCoffer that allows reading the continous memory range 
// between basePtr and basePtr + sz
//
// returns nil, os.EINVAL if basePtr is 0 or sz <= 0
//
func NewPtrCoffer(base_ uintptr, sz int) (coffer *PtrCoffer, err os.Error) {
    // base == 0 is interpreted as closed state
    if base_ == uintptr(0) {
        return nil, os.EINVAL
    }

    // sz must be positive
    if sz <= 0 {
        coffer = nil
        err = os.EINVAL
        return
    }

    seek_ := base_
    stop_ := uintptr(base_) + uintptr(sz-1)

    return &PtrCoffer{base: base_, seek: seek_, stop: stop_}, nil
}

func (p *PtrCoffer) InitPtrCoffer(base_ uintptr, sz int) os.Error {
    // base == 0 is interpreted as closed state
    if base_ == uintptr(0) {
        return os.EINVAL
    }

    // sz must be positive
    if sz <= 0 {
        return os.EINVAL
    }

    if p.IsOpen() {
        return os.EINVAL
    }

    p.base = base_
    p.seek = base_
    p.stop = uintptr(base_) + uintptr(sz-1)

    return nil
}

func (p *PtrCoffer) String() string {
    return fmt.Sprintf("&{base: %p, seek: %p, stop: %p} /* open := %t ; eof := %t; tell := %v; cap := %v */ ", p.base, p.seek, p.stop, p.IsOpen(), p.IsEOF(), p.Tell(), p.Cap())
}


// Current Seek position
func (p *PtrCoffer) Tell() int64 {
    if p.IsEOF() {
        return int64(p.Cap())
    }
    return int64(p.seek - p.base)
}

// Cap() - 1
func (p *PtrCoffer) Diff() int {
    return int(p.stop - p.base)
}

// Cap of the managed range, always >= 1
func (p *PtrCoffer) Cap() int {
    return int(p.stop-p.base) + 1
}

// Remaing bytes to be read or written
func (p *PtrCoffer) Len() int {
    if p.IsOpen() && !p.IsEOF() {
        return int(p.stop-p.seek) + 1
    }
    return 0
}

// true, iff EOF was encountered during a previous Read() or Write() call
//
// Seek() resets to false
func (p *PtrCoffer) IsEOF() bool {
    return (p.seek == uintptr(0))
}

// true, iff offset is contained in managed memory range
func (p *PtrCoffer) ContainsOffset(offset int64) bool {
    return offset >= 0 && offset <= int64(p.Diff())
}

// panic(os.EINVAL) iff offset is not contained in managed memory range
func (p *PtrCoffer) EnsureContainsOffset(offset int64) {
    if p.ContainsOffset(offset) {
        return
    }
    // else
    panic(os.EINVAL)
}

// true iff pos is contained in memory range
func (p *PtrCoffer) Contains(pos uintptr) bool {
    return (pos >= p.base && pos <= p.stop)
}

// panic(os.EINVAL) iff pos is not contained in memory range
func (p *PtrCoffer) EnsureContains(pos uintptr) {
    if p.Contains(pos) {
        return
    }
    // else
    panic(os.EINVAL)
}

// Compute an absolute seek position within this PtrCoffer
// (Parameters as in io.Seek)
//
// returns int64(p.seek), os.EINVAL iff whence is not in 0..2
func (p *PtrCoffer) SeekPos(offset int64, whence int) (ret int64, err os.Error) {
    var newOffset int64
    switch whence {
    default:
        return p.Tell(), os.EINVAL
    case 0:
        newOffset = offset
    case 1:
        newOffset = p.Tell() + offset
    case 2:
        newOffset = int64(p.Diff()) + offset
    }
    return newOffset, nil
}

// If offset points outside the underlying managed memory range
// returns p.seek, os.EINVAL
//
// If !p.IsOpen() returns p.seek, os.EINVAL
func (p *PtrCoffer) Seek(offset int64, whence int) (ret int64, err os.Error) {
    if !p.IsOpen() {
        return int64(p.seek), os.EINVAL
    }
    ret, err = p.SeekPos(offset, whence)
    if err != nil {
        return ret, err
    }
    p.EnsureContainsOffset(ret)
    p.seek = p.base + uintptr(ret)
    return
}

func (p *PtrCoffer) Read(dst []uint8) (n int, err os.Error) {
    // Bail out if EOF was hit before
    if !p.IsOpen() || p.IsEOF() {
        return 0, os.EOF
    }

    // Ensure copy only if dstLen > 0
    dstLen := len(dst)
    if dstLen == 0 {
        return 0, os.EINVAL
    }

    // Ensures copy only if srcLen > 0
    srcLen := p.Len()
    if srcLen == 0 {
        return 0, os.EINVAL
    }

    // Copy min(dstLen, srcLen) > 0 bytes
    srcPtr := unsafe.Pointer(p.seek)
    dstPtr := unsafe.Pointer(&dst[0])
    if srcLen > dstLen {
        C.memmove(dstPtr, srcPtr, C.size_t(dstLen))
        p.seek = p.seek + uintptr(dstLen)
        return int(dstLen), nil
    }
    // else srcLen <= dstLen
    C.memmove(dstPtr, srcPtr, C.size_t(srcLen))
    // Mark EOF
    p.seek = uintptr(0)
    return int(srcLen), os.EOF
}

// Will not append but instead stop with EOF at end of range
func (p *PtrCoffer) Write(src []uint8) (n int, err os.Error) {

    // Bail out if EOF was hit before
    if !p.IsOpen() || p.IsEOF() {
        return 0, os.EOF
    }

    // Ensure copy only if srcLen > 0
    // assumes sizeof(uintptr) >= sizeof(int) which is the case
    srcLen := len(src)
    if srcLen == 0 {
        return 0, os.EINVAL
    }

    // Ensures copy only if dstLen > 0
    dstLen := p.Len()
    if dstLen == 0 {
        return 0, os.EINVAL
    }

    // Copy min(dstLen, srcLen) > 0 bytes
    srcPtr := unsafe.Pointer(&src[0])
    dstPtr := unsafe.Pointer(p.seek)
    if srcLen >= dstLen {
        C.memmove(dstPtr, srcPtr, C.size_t(dstLen))
        // Mark EOF
        p.seek = uintptr(0)
        return int(dstLen), os.EOF
    }
    // else srcLen < dstLen
    C.memmove(dstPtr, srcPtr, C.size_t(srcLen))
    p.seek = p.seek + uintptr(srcLen)
    return int(srcLen), nil
}

func (p *PtrCoffer) IsOpen() bool {
    return (p.base != uintptr(0))
}

// Closes this coffer by zeroing all internal fields
//
// Cant Read() or Write() or Seek() anymore afterwards
//
// Does not free any managed pointers
func (p *PtrCoffer) Close() os.Error {
    // Zero ptrs to avoid any lingering harm
    p.base = uintptr(0)
    p.seek = uintptr(0)
    p.stop = uintptr(0)
    return nil
}

// Retrieve base as uintptr
func (p *PtrCoffer) GetBasePtr() uintptr {
	return p.base
}

// Retrieve seek uintptr
func (p *PtrCoffer) GetSeekPtr() uintptr {
	return p.base
}

// Retrieve stop as uintptr
func (p *PtrCoffer) GetStopPtr() uintptr {
	return p.base
}


// Selfallocating coffer via malloc, frees on Close()
type MemCoffer struct {
    PtrCoffer
}

// Allocate a coffer independent from the go runtime, i.e. 
// you are responsible for freeing its mem content
// by calling Close() (You get memory leaks iff you don't)
func NewMemCoffer(sz int) (coffer Coffer, err os.Error) {
    if sz < 0 {
        return nil, os.EINVAL
    }
    base_ := uintptr(C.malloc(C.size_t(sz)))
    if IsCNullPtr(base_) {
        return nil, os.Errno(GetCErrno())
    }
    seek_ := base_
    stop_ := uintptr(base_) + uintptr(sz-1)

    cf := new(MemCoffer)
    cf.base = base_
    cf.seek = seek_
    cf.stop = stop_
    coffer = cf
    return coffer, nil
}

func (p *MemCoffer) Close() os.Error {
    if p.IsEOF() {
        return os.EOF
    }
    // Free memory in defer
    backup := unsafe.Pointer(p.base)
    defer C.free(backup)

    // Zero ptrs to avoid any lingering harm
    p.base = uintptr(0)
    p.seek = uintptr(0)
    p.stop = uintptr(0)
    return nil
}

// {}
