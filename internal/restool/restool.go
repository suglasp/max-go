// restool.go : lists, unpacks, and packs Interplay RES0 archives.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package restool

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/suglasp/max-go/internal/resfile"
)

const manifestName = "_res_manifest.txt"

/* // note: unused code in Go port
// the RES0 layout repeated resfile; restool reads resfile.HeaderSize / EntrySize / Magic.
const (
	headerSize = 12
	entrySize  = 16
	magic      = "RES0"
)
*/

// ManifestName is the metadata file used to preserve archive order during unpack and repack operations.
const ManifestName = manifestName

// DefaultUnpackDir is the shared default output and input directory for max-restool and max-convert.
const DefaultUnpackDir = "extracted"

// Entry describes one resource in archive order.
type Entry struct {
	Tag  string
	Size int
}

type manifestEntry struct {
	Tag  string
	File string
	Hash string
}

/* // note: unused code in Go port
// ManifestEntry was only returned by the commented out ReadManifest; Pack reads the manifest rows itself.
// ManifestEntry pairs a resource tag with the raw file containing its bytes.
type ManifestEntry struct {
	Tag  string
	File string
}
*/

// convertedExts lists max-convert output files that Pack ignores when no manifest is available.
var convertedExts = map[string]bool{
	".png": true, ".txt": true, ".bmp": true, ".wav": true,
	".ttf": true, ".otf": true, ".ttc": true,
}

// List returns all archive entries in index order.
func List(path string) ([]Entry, error) {
	rf, err := resfile.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer rf.Close()

	tags := rf.Tags()
	out := make([]Entry, 0, len(tags))
	for _, tag := range tags {
		size, ok := rf.Size(tag)
		if !ok {
			return nil, fmt.Errorf("restool: %s: tag %q disappeared while listing", path, tag)
		}
		out = append(out, Entry{Tag: tag, Size: size})
	}
	return out, nil
}

// Unpack writes each resource to dir and records archive order in a manifest.
func Unpack(path, dir string) error {
	rf, err := resfile.OpenFile(path)
	if err != nil {
		return err
	}
	defer rf.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	used := map[string]struct{}{}
	var manifest []manifestEntry
	for _, tag := range rf.Tags() {
		data, err := rf.Read(tag)
		if err != nil {
			return err
		}
		name := uniqueName(safeFileName(tag), used)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		manifest = append(manifest, manifestEntry{
			Tag:  tag,
			File: name,
			Hash: hex.EncodeToString(sum[:]),
		})
	}
	return writeManifest(filepath.Join(dir, manifestName), manifest)
}

/* // note: unused code in Go port
// ReadManifest returns tags and filenames in archive order. Integrity hashes remain private to Pack.
func ReadManifest(dir string) ([]ManifestEntry, error) {
	entries, err := readManifest(filepath.Join(dir, manifestName))
	if err != nil {
		return nil, err
	}
	out := make([]ManifestEntry, len(entries))
	for i, entry := range entries {
		out[i] = ManifestEntry{Tag: entry.Tag, File: entry.File}
	}
	return out, nil
}
*/

// Pack writes a RES0 archive from dir. A manifest preserves tag order and filenames; without one, files are
// sorted and their names become normalized tags.
func Pack(dir, outPath string) error {
	manifest, err := readManifest(filepath.Join(dir, manifestName))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		manifest, err = manifestFromDir(dir)
		if err != nil {
			return err
		}
	}
	if len(manifest) == 0 {
		return fmt.Errorf("restool: %s: no files to pack", dir)
	}
	return writeArchive(dir, outPath, manifest)
}

// writeArchive packs manifest entries from dir into a RES archive at outPath.
func writeArchive(dir, outPath string, manifest []manifestEntry) error {
	type payload struct {
		tag  string
		data []byte
	}
	payloads := make([]payload, 0, len(manifest))
	seen := map[string]struct{}{}
	for _, item := range manifest {
		tag := resfile.NormTag(item.Tag)
		if tag == "" || len(tag) > resfile.TagLen {
			return fmt.Errorf("restool: invalid tag %q", item.Tag)
		}
		if _, dup := seen[tag]; dup {
			return fmt.Errorf("restool: duplicate tag %q", tag)
		}
		seen[tag] = struct{}{}
		data, err := os.ReadFile(filepath.Join(dir, item.File))
		if err != nil {
			return err
		}
		if item.Hash != "" {
			sum := sha256.Sum256(data)
			if got := hex.EncodeToString(sum[:]); got != item.Hash {
				return fmt.Errorf("restool: %s: hash mismatch for %s", item.File, tag)
			}
		}
		payloads = append(payloads, payload{tag: tag, data: data})
	}

	var total int64 = resfile.HeaderSize + int64(len(payloads)*resfile.EntrySize)
	for _, p := range payloads {
		total += int64(len(p.data))
	}
	if total > int64(^uint32(0)) {
		return fmt.Errorf("restool: archive too large: %d bytes", total)
	}

	tmp := outPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmp)
		}
	}()

	if _, err := f.Write(make([]byte, resfile.HeaderSize)); err != nil {
		f.Close()
		return err
	}
	index := make([]byte, len(payloads)*resfile.EntrySize)
	for i, p := range payloads {
		pos, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			f.Close()
			return err
		}
		if pos > int64(^uint32(0)) || len(p.data) > int(^uint32(0)) {
			f.Close()
			return fmt.Errorf("restool: entry %s too large", p.tag)
		}
		rec := index[i*resfile.EntrySize : (i+1)*resfile.EntrySize]
		copy(rec[:resfile.TagLen], p.tag)
		binary.LittleEndian.PutUint32(rec[8:12], uint32(pos))
		binary.LittleEndian.PutUint32(rec[12:16], uint32(len(p.data)))
		if _, err := f.Write(p.data); err != nil {
			f.Close()
			return err
		}
	}
	indexOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(index); err != nil {
		f.Close()
		return err
	}
	var hdr [resfile.HeaderSize]byte
	copy(hdr[:4], resfile.Magic)
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(indexOffset))
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(len(index)))
	if _, err := f.WriteAt(hdr[:], 0); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, outPath); err != nil {
		return err
	}
	success = true
	return nil
}

// writeManifest saves tab-separated tag, filename, and SHA-256 records for later repacking.
func writeManifest(path string, entries []manifestEntry) error {
	var b strings.Builder
	b.WriteString("# max-go RES manifest v1\n")
	for _, e := range entries {
		b.WriteString(e.Tag)
		b.WriteByte('\t')
		b.WriteString(e.File)
		b.WriteByte('\t')
		b.WriteString(e.Hash)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// readManifest loads the tab-separated metadata written by writeManifest.
func readManifest(path string) ([]manifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []manifestEntry
	sc := bufio.NewScanner(f)
	for lineNum := 1; sc.Scan(); lineNum++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// The hash column is optional: a hand written manifest (such as the port's LANG.RES sources) may leave
		// it out so its text files can be edited without breaking the pack.
		parts := strings.Split(line, "\t")
		if len(parts) != 2 && len(parts) != 3 {
			return nil, fmt.Errorf("restool: %s:%d: expected tag, file[, hash]", path, lineNum)
		}
		entry := manifestEntry{Tag: parts[0], File: parts[1]}
		if len(parts) == 3 {
			entry.Hash = parts[2]
		}
		out = append(out, entry)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// manifestFromDir builds a deterministic manifest from files in a directory.
func manifestFromDir(dir string) ([]manifestEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var out []manifestEntry
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == manifestName {
			continue
		}
		// Ignore converted siblings such as TAG.png, TAG.txt, and _palette.png so only raw resources enter the archive.
		if convertedExts[strings.ToLower(filepath.Ext(entry.Name()))] {
			continue
		}
		tag := resfile.NormTag(entry.Name())
		if tag == "" {
			return nil, fmt.Errorf("restool: cannot derive tag from %q", entry.Name())
		}
		out = append(out, manifestEntry{Tag: tag, File: entry.Name()})
	}
	return out, nil
}

// safeFileName converts a RES tag into a portable unpack filename.
func safeFileName(tag string) string {
	tag = resfile.NormTag(tag)
	if tag == "" {
		return "EMPTY"
	}
	var b strings.Builder
	for len(tag) > 0 {
		r, n := utf8.DecodeRuneInString(tag)
		tag = tag[n:]
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// uniqueName appends a numeric suffix until name is unique within used.
func uniqueName(name string, used map[string]struct{}) string {
	if _, ok := used[name]; !ok {
		used[name] = struct{}{}
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d", name, i)
		if _, ok := used[candidate]; !ok {
			used[candidate] = struct{}{}
			return candidate
		}
	}
}
