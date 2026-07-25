// reader_test.go : testing unit for reader.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package maxsave

import "testing"

// fakeObj records one payload byte so tests can exercise object references without loading unit classes.
type fakeObj struct{ payload uint8 }

func (o *fakeObj) FileLoad(r *Reader) { o.payload = r.U8() }

func le16(v uint16) []byte { return []byte{byte(v), byte(v >> 8)} }

func TestReaderPrimitives(t *testing.T) {
	// version=70, then u8, i8, u16, i32, a string with a length prefix.
	data := []byte{70, 0} // version
	data = append(data, 0xAB)
	data = append(data, 0xFF) // i8 = -1
	data = append(data, 0x34, 0x12)
	data = append(data, 0xFF, 0xFF, 0xFF, 0xFF) // i32 = -1
	data = append(data, 3, 0, 0, 0, 'a', 'b', 'c')

	r := NewReader(data, nil)
	if r.Format() != FormatV70 {
		t.Fatalf("format = %d, want 70", r.Format())
	}
	if got := r.U16(); got != 70 {
		t.Fatalf("version u16 = %d, want 70", got)
	}
	if got := r.U8(); got != 0xAB {
		t.Errorf("u8 = %#x, want 0xAB", got)
	}
	if got := r.I8(); got != -1 {
		t.Errorf("i8 = %d, want -1", got)
	}
	if got := r.U16(); got != 0x1234 {
		t.Errorf("u16 = %#x, want 0x1234", got)
	}
	if got := r.I32(); got != -1 {
		t.Errorf("i32 = %d, want -1", got)
	}
	if got := r.String(); got != "abc" {
		t.Errorf("string = %q, want abc", got)
	}
	if r.Err() != nil {
		t.Fatalf("unexpected err: %v", r.Err())
	}
}

func TestReaderShortReadIsSticky(t *testing.T) {
	r := NewReader([]byte{70, 0, 0x01}, nil) // version + 1 stray byte
	_ = r.U16()                              // version
	_ = r.U32()                              // wants 4, only 1 left -> error
	if r.Err() == nil {
		t.Fatal("expected a short-read error")
	}
	// Further reads do nothing and return zero.
	if got := r.U16(); got != 0 {
		t.Errorf("post-error read = %d, want 0", got)
	}
}

func TestReadObjectGraph(t *testing.T) {
	reg := map[uint32]Allocator{
		1: func() FileObject { return &fakeObj{} },
	}

	// Stream (after a two byte version the reader ignores for Index width):
	//  [00 00]            -> null object
	//  [01 00][01 00][AA] -> new object #1, type 1, payload 0xAA
	//  [01 00]            -> back reference to object #1
	data := []byte{70, 0}
	data = append(data, le16(0)...) // null
	data = append(data, le16(1)...) // new index 1
	data = append(data, le16(1)...) // type 1
	data = append(data, 0xAA)       // payload
	data = append(data, le16(1)...) // back reference to 1

	r := NewReader(data, reg)
	_ = r.U16() // consume version

	if o := r.ReadObject(); o != nil {
		t.Fatalf("null object = %v, want nil", o)
	}
	o1 := r.ReadObject()
	f1, ok := o1.(*fakeObj)
	if !ok || f1.payload != 0xAA {
		t.Fatalf("new object = %+v, want fakeObj payload 0xAA", o1)
	}
	o2 := r.ReadObject()
	if o2 != o1 {
		t.Fatalf("back-reference returned %p, want the same object %p", o2, o1)
	}
	if r.Err() != nil {
		t.Fatalf("unexpected err: %v", r.Err())
	}
	if r.Pos() != len(data) {
		t.Errorf("consumed %d bytes, want %d (back-ref must not read a payload)", r.Pos(), len(data))
	}
}

func TestReadObjectUnknownType(t *testing.T) {
	data := []byte{70, 0}
	data = append(data, le16(1)...) // new index 1
	data = append(data, le16(9)...) // type 9 (unregistered)
	r := NewReader(data, map[uint32]Allocator{})
	_ = r.U16()
	if o := r.ReadObject(); o != nil || r.Err() == nil {
		t.Fatalf("unknown type should fail: obj=%v err=%v", o, r.Err())
	}
}
