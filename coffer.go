package coffer

import . "unsafe"
import os "os"

// #include <stdlib.h>
// #include <string.h>
import "C"

// Transplants ReadWriteSeeker interface on unsafe.Pointer
// 
// This is inherently not thread-safe
type Coffer struct {
  start uintptr // base pointer
  seek  uintptr // offset value
  limit uintptr // pointer to last element
  diff  uintptr // limit-start

  // cant use seek > limit to test for eof due to potential overflow issues

  eof bool // if set eof, has been reached, reset via Seak() issues
  fin bool // if set, coffer has been closed, subsequent Read, Write, Seek will fail
}

func NewCoffer(startPtr Pointer, limitPtr Pointer) (coffer *Coffer, err os.Error) {
  start_ := uintptr(startPtr)
  limit_ := uintptr(limitPtr)
  diff_ := limit_ - start_
  // Makes life easier by avoiding that diff-start+1 could overflow
  // Check Read() and Write()!
  if start_ == uintptr(0) {
    return nil, os.EINVAL
  }
  if start_ > limit_ {
    coffer = nil
    err = os.EINVAL
    return
  }
  return &Coffer{start: start_, limit: limit_, diff: diff_}, nil
}

func (p *Coffer) Tell() int64 { return int64(p.seek) }

func (p *Coffer) Diff() uintptr { return p.diff }

func (p *Coffer) Size() int64 { return int64(p.diff) + 1 }

func (p *Coffer) IsEOF() bool { return p.eof }

func (p *Coffer) ContainsOffset(offset int64) bool {
  return offset >= 0 && offset <= int64(p.diff)
}

func (p *Coffer) EnsureContainsOffset(offset int64) {
  if p.ContainsOffset(offset) {
    return
  }
  // else
  panic(os.EINVAL)
}

func (p *Coffer) Contains(pos uintptr) bool {
  return (pos >= p.start && pos <= p.limit)
}

func (p *Coffer) EnsureContains(pos uintptr) {
  if p.Contains(pos) {
    return
  }
  // else
  panic(os.EINVAL)
}

// Compute an absolute seek position within this Coffer
func (p *Coffer) SeekPos(whence int, offset int64) (ret int64, err os.Error) {
  var newOffset int64
  switch whence {
  default:
    return int64(p.seek), os.EINVAL
  case 0:
    newOffset = offset
  case 1:
    newOffset = int64(p.seek) + offset
  case 2:
    newOffset = int64(p.diff) + offset
  }
  return newOffset, nil
}

// Regular seek except that you cannot append or prepend
// (seek before start or behind the end)
//
// Clears EOF state
//
// If offset lies outside returns current seek, os.EINVAL
func (p *Coffer) Seek(whence int, offset int64) (ret int64, err os.Error) {
	if p.fin {
		return int64(p.seek), os.EINVAL
	}
  ret, err = p.SeekPos(whence, offset)
  p.EnsureContainsOffset(ret)
  p.seek = uintptr(ret)
  p.eof = false
  return
}

func (p *Coffer) Read(dst []uint8) (n int, err os.Error) {
  // Bail out if EOF was hit before
  if p.eof || p.fin {
    return 0, os.EOF
  }

  // Ensure copy only if dstCap > 0
  // assumes sizeof(uintptr) >= sizeof(int) which is the case
  dstCap := uintptr(len(dst))
  if dstCap == 0 {
    return 0, os.EINVAL
  }

  // Ensures copy only if srcCap > 0
  var srcCap uintptr = uintptr(p.diff-p.seek) + uintptr(1)
  if srcCap == 0 {
    return 0, os.EINVAL
  }

  // Copy min(dstCap, srcCap) > 0 bytes
  srcPtr := Pointer(uintptr(p.start) + uintptr(p.seek))
  dstPtr := Pointer(&dst[0])
  if srcCap > dstCap {
    C.memmove(dstPtr, srcPtr, C.size_t(dstCap))
    p.seek = p.seek + dstCap
    return int(dstCap), nil
  }
  // else srcCap <= dstCap
  C.memmove(dstPtr, srcPtr, C.size_t(srcCap))
  p.seek = p.diff
  p.eof = true
  return int(srcCap), os.EOF
}

func (p *Coffer) Write(src []uint8) (n int, err os.Error) {
  // Bail out if EOF was hit before
  if p.eof || p.fin {
    return 0, os.EOF
  }

  // Ensure copy only if srcCap > 0
  // assumes sizeof(uintptr) >= sizeof(int) which is the case
  srcCap := uintptr(len(src))
  if srcCap == 0 {
    return 0, os.EINVAL
  }

  // Ensures copy only if dstCap > 0
  var dstCap uintptr = uintptr(p.diff-p.seek) + uintptr(1)
  if dstCap == 0 {
    return 0, os.EINVAL
  }

  // Copy min(dstCap, srcCap) > 0 bytes
  srcPtr := Pointer(&src[0])
  dstPtr := Pointer(uintptr(p.start) + uintptr(p.seek))
  if srcCap >= dstCap {
    C.memmove(dstPtr, srcPtr, C.size_t(dstCap))
    p.seek = p.diff
    p.eof = true
    return int(dstCap), os.EOF
  }
  // else srcCap < dstCap
  C.memmove(dstPtr, srcPtr, C.size_t(srcCap))
  p.seek = p.seek + srcCap
  return int(srcCap), nil
}

func (p *Coffer) Close() os.Error {
	// Zero's ptrs to avoid any lingering harm
	p.start = uintptr(0)
	p.limit = uintptr(0)
	p.diff  = uintptr(0)
	p.seek  = 0
	p.eof   = true
	p.fin   = true
	return nil
}

// {}
