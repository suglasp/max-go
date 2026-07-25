// resfile.go : the M.A.X. RES archive reader - reads *.RES files (mainly 'MAX.RES') with layered override support.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package resfile reads Interplay RES archives as documented in docs/formats/res.md.
// A Library searches archives in opening order, allowing PATCHES.RES to override matching resources in MAX.RES.
package resfile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	headerSize = 12     // id[4] + index offset + index size
	entrySize  = 16     // tag[8] + data offset + data size
	magic      = "RES0" // Magic header
	TagLen     = 8      // TagLen is the maximum length of a resource tag.
)

type span struct {
	offset int32
	size   int32
}

// File is a single opened RES archive.
type File struct {
	path    string
	f       *os.File
	entries map[string]span
	tags    []string // Tags in archive-index order.
}

// OpenFile opens and indexes one RES archive.
//
// C++: ResourceManager_BuildResourceTable (resource_manager.cpp)
func OpenFile(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	rf, err := index(f, path)
	if err != nil {
		f.Close()
		return nil, err
	}
	return rf, nil
}

// index reads a RES directory table and records each entry's file offset.
func index(f *os.File, path string) (*File, error) {
	var hdr [headerSize]byte
	if _, err := f.ReadAt(hdr[:], 0); err != nil {
		return nil, fmt.Errorf("resfile: %s: read header: %w", path, err)
	}
	if string(hdr[:4]) != magic {
		return nil, fmt.Errorf("resfile: %s: bad magic %q, want %q", path, hdr[:4], magic)
	}
	indexOffset := int32(binary.LittleEndian.Uint32(hdr[4:8]))
	indexSize := int32(binary.LittleEndian.Uint32(hdr[8:12]))

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if indexOffset < headerSize || indexSize < 0 || indexSize%entrySize != 0 ||
		int64(indexOffset)+int64(indexSize) > st.Size() {
		return nil, fmt.Errorf("resfile: %s: corrupt index (offset=%d size=%d filesize=%d)",
			path, indexOffset, indexSize, st.Size())
	}

	raw := make([]byte, indexSize)
	if _, err := f.ReadAt(raw, int64(indexOffset)); err != nil {
		return nil, fmt.Errorf("resfile: %s: read index: %w", path, err)
	}

	n := int(indexSize) / entrySize
	rf := &File{
		path:    path,
		f:       f,
		entries: make(map[string]span, n),
		tags:    make([]string, 0, n),
	}
	for i := 0; i < n; i++ {
		rec := raw[i*entrySize : (i+1)*entrySize]
		tag := NormTag(string(rec[:TagLen]))
		if tag == "" {
			continue
		}
		sp := span{
			offset: int32(binary.LittleEndian.Uint32(rec[8:12])),
			size:   int32(binary.LittleEndian.Uint32(rec[12:16])),
		}
		if sp.offset < 0 || sp.size < 0 || int64(sp.offset)+int64(sp.size) > st.Size() {
			return nil, fmt.Errorf("resfile: %s: entry %q out of bounds (offset=%d size=%d)",
				path, tag, sp.offset, sp.size)
		}
		// First occurrence wins, matching the original loader.
		if _, dup := rf.entries[tag]; dup {
			continue
		}
		rf.entries[tag] = sp
		rf.tags = append(rf.tags, tag)
	}
	return rf, nil
}

// NormTag trims an archive tag at its first NUL, limits it to TagLen bytes, removes surrounding spaces, and converts it to uppercase.
func NormTag(tag string) string {
	if i := strings.IndexByte(tag, 0); i >= 0 {
		tag = tag[:i]
	}
	if len(tag) > TagLen {
		tag = tag[:TagLen]
	}
	return strings.ToUpper(strings.TrimSpace(tag))
}

// Path returns the file path the archive was opened from.
func (f *File) Path() string {
	return f.path
}

// Tags returns all tags in index order.
func (f *File) Tags() []string {
	return append([]string(nil), f.tags...)
}

// Has reports whether the archive contains tag.
func (f *File) Has(tag string) bool {
	_, ok := f.entries[NormTag(tag)]
	return ok
}

// Size returns the stored size of the resource, or ok=false if absent.
func (f *File) Size(tag string) (size int, ok bool) {
	sp, ok := f.entries[NormTag(tag)]
	return int(sp.size), ok
}

// Read returns a copy of the resource data for tag.
//
// C++: ResourceManager_ReadResource (resource_manager.cpp)
func (f *File) Read(tag string) ([]byte, error) {
	sp, ok := f.entries[NormTag(tag)]
	if !ok {
		return nil, fmt.Errorf("resfile: %s: no resource %q", f.path, NormTag(tag))
	}
	buf := make([]byte, sp.size)
	if _, err := f.f.ReadAt(buf, int64(sp.offset)); err != nil {
		return nil, fmt.Errorf("resfile: %s: read %q: %w", f.path, NormTag(tag), err)
	}
	return buf, nil
}

// Close closes the underlying file.
func (f *File) Close() error {
	return f.f.Close()
}

// Library is an ordered stack of RES archives. Lookups return the first matching resource.
type Library struct {
	files []*File
}

// Open loads archives in precedence order. Pass PATCHES.RES before MAX.RES.
func Open(paths ...string) (*Library, error) {
	lib := &Library{}
	for _, p := range paths {
		f, err := OpenFile(p)
		if err != nil {
			lib.Close()
			return nil, err
		}
		lib.files = append(lib.files, f)
	}
	return lib, nil
}

// OpenDir loads optional PATCHES.RES over required MAX.RES from a game-data directory. It matches filenames without case sensitivity.
func OpenDir(dir string) (*Library, error) {
	maxRes, err := findFile(dir, "MAX.RES")
	if err != nil {
		return nil, fmt.Errorf("resfile: MAX.RES not found in %s. Set data_dir in max-go.json to the M.A.X. game data folder: %w", dir, err)
	}
	paths := []string{}
	if patches, err := findFile(dir, "PATCHES.RES"); err == nil {
		paths = append(paths, patches)
	}
	paths = append(paths, maxRes)
	return Open(paths...)
}

// findFile locates name in dir, matching without case sensitivity.
func findFile(dir, name string) (string, error) {
	exact := filepath.Join(dir, name)
	if _, err := os.Stat(exact); err == nil {
		return exact, nil
	}
	list, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range list {
		if !e.IsDir() && strings.EqualFold(e.Name(), name) {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", os.ErrNotExist
}

// Files returns the layered archives in precedence order.
func (l *Library) Files() []*File {
	return append([]*File(nil), l.files...)
}

// Has reports whether any archive contains tag.
func (l *Library) Has(tag string) bool {
	for _, f := range l.files {
		if f.Has(tag) {
			return true
		}
	}
	return false
}

// Size returns the stored size from the first archive containing tag.
func (l *Library) Size(tag string) (size int, ok bool) {
	for _, f := range l.files {
		if s, ok := f.Size(tag); ok {
			return s, true
		}
	}
	return 0, false
}

// Read returns the resource data from the first archive that has it.
func (l *Library) Read(tag string) ([]byte, error) {
	for _, f := range l.files {
		if f.Has(tag) {
			return f.Read(tag)
		}
	}
	return nil, fmt.Errorf("resfile: no resource %q in %d archive(s)", NormTag(tag), len(l.files))
}

// ReadString returns a resource as text without trailing NUL bytes.
func (l *Library) ReadString(tag string) (string, error) {
	data, err := l.Read(tag)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimRight(data, "\x00")), nil
}

// Close closes all archives, returning the first error.
func (l *Library) Close() error {
	var first error
	for _, f := range l.files {
		if err := f.Close(); err != nil && first == nil {
			first = err
		}
	}
	l.files = nil
	return first
}
