// reader.go : SmartFileReader core; little endian primitive reads and the shared object graph for the retail V70 game state format.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : smartfile.cpp - SmartFileReader
package maxsave

import (
	"encoding/binary"
	"fmt"
)

// Format is the SmartFile version. Retail mission/save files are V70; V71 is a
// newer format introduced by the C++ port this project is based on. The two
// differ in the width of object indices and list counts (V70 = uint16, V71 = uint32).
type Format uint16

const (
	FormatV70 Format = 70
	FormatV71 Format = 71
)

// FileObject is a value that can deserialize itself from a SmartFile stream,
// the Go equivalent of the C++ FileObject base. Implementations read exactly the
// bytes their C++ FileLoad consumes so the stream stays aligned.
//
// C++ ported : fileobject.hpp - FileObject::FileLoad
type FileObject interface {
	FileLoad(r *Reader)
}

// Allocator constructs a fresh FileObject for a registered class.
type Allocator func() FileObject

// Reader parses SmartFile V70/V71 data from memory and maintains the shared object table used to resolve references. Err reports the first read failure.
type Reader struct {
	data    []byte
	pos     int
	format  Format
	objects []FileObject // read_objects: 1-based, in first encounter order
	reg     map[uint32]Allocator
	err     error
}

// NewReader wraps data, taking the format from the leading version word (the
// file's first field, which the caller reads again as part of the body). reg maps
// each class's registration type index to its allocator.
//
// C++ ported : smartfile.cpp - SmartFileReader::Open (format probe)
func NewReader(data []byte, reg map[uint32]Allocator) *Reader {
	f := FormatV70
	if len(data) >= 2 {
		f = Format(binary.LittleEndian.Uint16(data))
	}
	return &Reader{data: data, format: f, reg: reg}
}

// Format reports the detected SmartFile version.
func (r *Reader) Format() Format {
	return r.format
}

// Err returns the first error encountered (a short read or a bad object/type
// index). Once set, subsequent reads do nothing and return zero.
func (r *Reader) Err() error {
	return r.err
}

// Pos returns the current byte offset.
func (r *Reader) Pos() int {
	return r.pos
}

// fail records the first error.
func (r *Reader) fail(format string, args ...any) {
	if r.err == nil {
		r.err = fmt.Errorf("maxsave: "+format, args...)
	}
}

// take returns the next n bytes and advances, or sets err and returns nil.
func (r *Reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || r.pos+n > len(r.data) {
		r.fail("short read: want %d bytes at offset %d of %d", n, r.pos, len(r.data))
		return nil
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b
}

// Skip advances over n bytes (e.g. a field we don't model).
func (r *Reader) Skip(n int) {
	r.take(n)
}

// Bytes reads n raw bytes into a fresh slice.
func (r *Reader) Bytes(n int) []byte {
	b := r.take(n)
	if b == nil {
		return nil
	}
	out := make([]byte, n)
	copy(out, b)
	return out
}

// U8/I8/U16/I16/U32/I32 read one little endian scalar.
func (r *Reader) U8() uint8 {
	b := r.take(1)
	if b == nil {
		return 0
	}
	return b[0]
}

func (r *Reader) I8() int8 {
	return int8(r.U8())
}

func (r *Reader) U16() uint16 {
	b := r.take(2)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint16(b)
}

func (r *Reader) I16() int16 {
	return int16(r.U16())
}

func (r *Reader) U32() uint32 {
	b := r.take(4)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(b)
}

func (r *Reader) I32() int32 {
	return int32(r.U32())
}

// Bool reads a one byte boolean.
func (r *Reader) Bool() bool {
	return r.U8() != 0
}

// Index reads an object or back reference index: uint16 on V70, uint32 on V71.
//
// C++ ported : smartfile.cpp - SmartFileReader::ReadIndex
func (r *Reader) Index() uint32 {
	if r.format == FormatV70 {
		return uint32(r.U16())
	}
	return r.U32()
}

// Count reads a list element count, with the same width as Index.
//
// C++ ported : smartfile.cpp - SmartFileReader::ReadObjectCount
func (r *Reader) Count() uint32 {
	if r.format == FormatV70 {
		return uint32(r.U16())
	}
	return r.U32()
}

// String reads a length prefixed string (uint32 length, then bytes).
//
// C++ ported : smartfile.hpp - SmartFileReader::Read(std::string&)
func (r *Reader) String() string {
	n := r.U32()
	if n == 0 {
		return ""
	}
	return string(r.Bytes(int(n)))
}

// ReadObject resolves the next shared-object reference. Zero is nil, an existing index is a back reference, and the next sequential index introduces a typed object.
// New objects enter the table before FileLoad runs so references encountered during loading can resolve them.
//
// C++ ported : smartfile.cpp - SmartFileReader::ReadObject
func (r *Reader) ReadObject() FileObject {
	idx := r.Index()
	if idx == 0 || r.err != nil {
		return nil
	}
	if int(idx) <= len(r.objects) {
		return r.objects[idx-1] // back reference
	}
	if int(idx) != len(r.objects)+1 {
		r.fail("object index %d out of sequence (table has %d)", idx, len(r.objects))
		return nil
	}
	typeIdx := r.Index()
	alloc := r.reg[typeIdx]
	if alloc == nil {
		r.fail("unknown object type index %d at offset %d", typeIdx, r.pos)
		return nil
	}
	obj := alloc()
	r.objects = append(r.objects, obj) // insert before FileLoad
	obj.FileLoad(r)
	return obj
}

// ReadObjectTyped reads a shared object whose class the FILE decides: the stored class
// index picks the reader out of the registry. An index the registry does not know falls
// back to the caller's allocator rather than failing the whole file, so an unexpected
// class degrades to a guess instead of a dead save.
//
// This is the slot where a unit's path lives, and it can be any of three classes with
// three different layouts (ground, air, builder). Picking one from the unit's flags
// misreads a builder's path and drags every later object in the file out of alignment.
//
// C++ ported : smartfile.cpp - SmartFileReader::ReadObject (RegisterClass[type_index - 1].Allocate)
func (r *Reader) ReadObjectTyped(fallback Allocator) FileObject {
	idx := r.Index()
	if idx == 0 || r.err != nil {
		return nil
	}

	if int(idx) <= len(r.objects) {
		return r.objects[idx-1] // back reference
	}

	if int(idx) != len(r.objects)+1 {
		r.fail("object index %d out of sequence (table has %d)", idx, len(r.objects))
		return nil
	}

	typeIdx := r.Index()

	alloc := r.reg[typeIdx]
	if alloc == nil {
		alloc = fallback
	}

	obj := alloc()
	r.objects = append(r.objects, obj) // insert before FileLoad, so a self reference resolves
	obj.FileLoad(r)

	return obj
}

// ReadObjectAs is ReadObject for callers that know the concrete type from
// context (e.g. a TeamUnits base value slot is always a UnitValues). It uses
// the supplied allocator instead of the type registry, but still consumes the
// stored type index, so it works without knowing the file's class registration
// order. Back references and nulls behave as in ReadObject.
func (r *Reader) ReadObjectAs(alloc Allocator) FileObject {
	idx := r.Index()
	if idx == 0 || r.err != nil {
		return nil
	}
	if int(idx) <= len(r.objects) {
		return r.objects[idx-1] // back reference
	}
	if int(idx) != len(r.objects)+1 {
		r.fail("object index %d out of sequence (table has %d)", idx, len(r.objects))
		return nil
	}
	_ = r.Index() // stored type index: the caller's allocator overrides it
	obj := alloc()
	r.objects = append(r.objects, obj)
	obj.FileLoad(r)
	return obj
}
