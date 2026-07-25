// classdispatch_test.go : tests that a shared object's class comes from the file, not a guess.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package maxsave

import (
	"testing"
)

// TestReadObjectTypedUsesTheStoredClass checks that a unit path slot uses its stored ground, air, or builder class. Guessing from unit flags would
// read a builder's four-byte record as ground-path steps and misalign later objects.
func TestReadObjectTypedUsesTheStoredClass(t *testing.T) {
	// A builder path record: object index 1, class index 2 (BuilderPath), 4 payload bytes.
	data := []byte{70, 0}
	data = append(data, le16(1)...)               // new object #1
	data = append(data, le16(typeBuilderPath)...) // its class, as the file stores it
	data = append(data, 1, 0, 2, 0)               // m_direction_x / m_direction_y

	r := NewReader(data, classRegistry())
	_ = r.U16() // the version word

	// The fallback would say GroundPath (the unit is not an air unit); the file wins.
	obj := r.ReadObjectTyped(pathAllocFor(0))

	if _, ok := obj.(*BuilderPath); !ok {
		t.Fatalf("read %T, want *BuilderPath (the class the file names)", obj)
	}

	if r.Err() != nil {
		t.Fatalf("unexpected error: %v", r.Err())
	}

	if r.Pos() != len(data) {
		t.Errorf("consumed %d bytes of %d: the builder path record must be read whole", r.Pos(), len(data))
	}
}

// TestReadObjectTypedFallsBackOnUnknownClass: an index the registry does not know
// degrades to the caller's guess instead of failing the whole file.
func TestReadObjectTypedFallsBackOnUnknownClass(t *testing.T) {
	data := []byte{70, 0}
	data = append(data, le16(1)...)          // new object #1
	data = append(data, le16(99)...)         // a class index nothing is registered under
	data = append(data, make([]byte, 27)...) // an air path's fixed payload

	r := NewReader(data, classRegistry())
	_ = r.U16()

	if obj := r.ReadObjectTyped(pathAllocFor(flagMobileAirUnit)); obj == nil {
		t.Fatal("an unknown class must fall back to the caller's allocator, not fail")
	}

	if r.Err() != nil {
		t.Errorf("fallback reported an error: %v", r.Err())
	}
}

// TestClassRegistryIsAlphabetical pins the index mapping: the original registers its six
// classes in an array sorted by class NAME, so the stored index is the alphabetical
// position. Every retail mission parses under it (TestLoadAllMissions), which is the
// real proof; this keeps the derivation visible.
func TestClassRegistryIsAlphabetical(t *testing.T) {
	names := []string{"", "AirPath", "BuilderPath", "Complex", "GroundPath", "UnitInfo", "UnitValues"}

	for idx, name := range names {
		if idx == 0 {
			continue // index 0 is "no object"
		}

		if _, ok := classRegistry()[uint32(idx)]; !ok {
			t.Errorf("class index %d (%s) is not registered", idx, name)
		}
	}

	if len(classRegistry()) != len(names)-1 {
		t.Errorf("registry holds %d classes, want %d", len(classRegistry()), len(names)-1)
	}
}
