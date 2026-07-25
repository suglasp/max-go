// dbg.go : provides console and file logging with module and team tags.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package dbg reads its controls from the debug section of settings.json.
// debug.console enables all output and writes each line to stderr. When
// debug.log_file is also set, the same lines are buffered to a file and flushed
// on Close.
// Each line is formatted "[DD-MM-YY@HH:mm:ss] [<MODULE>] [<TEAM>] : <message>",
// where the module and team brackets are optional.
// Fatal errors do not go through this package, they are always reported.
package dbg

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Module tags for Log / Logf: the subsystem a line comes from, UPPERCASE so a
// log is greppable by subsystem (grep "[SFX]" max-go.log). Extend as needed; a
// line for one unit passes the uppercased unit type name as the tag instead.
const (
	SFX    = "SFX"
	GFX    = "GFX"
	AI     = "AI"
	SIM    = "SIM"
	UI     = "UI"
	MOVIE  = "MOVIE"
	NET    = "NET"
	SAVE   = "SAVE"
	ECON   = "ECON"
	PATH   = "PATH"
	INPUT  = "INPUT"
	RES    = "RES"
	BUILD  = "BUILD"
	COMBAT = "COMBAT"
)

// timeLayout is the bracketed Go layout for the line timestamp: DD-MM-YY@HH:mm:ss
// (two digit year, 24h time, no milliseconds).
const timeLayout = "[02-01-06@15:04:05]"

// DefaultBufferLines is the log file batch size used when Init gets a
// zero or negative value.
const DefaultBufferLines = 10

var (
	mu sync.Mutex
	// console is the master switch: whether debug output is on at all. It is
	// atomic so Printf can return early when debugging is off without taking
	// the mutex; a disabled dbg.Printf then costs almost nothing (no lock, no format).
	console atomic.Bool
	logFile *bufferedFile // not nil only when console is on and a log path is set
)

// bufferedFile batches whole log lines and writes them to disk once maxLines
// have accumulated (or on flush/close), so frequent logging does not stall the
// game loop with a disk write per line.
type bufferedFile struct {
	f        *os.File
	pending  bytes.Buffer
	lines    int
	maxLines int
}

// write appends one formatted line, flushing the batch when it is full.
func (b *bufferedFile) write(line string) {
	b.pending.WriteString(line)
	b.lines++
	if b.lines >= b.maxLines {
		b.flush()
	}
}

// flush writes the batched lines to disk.
func (b *bufferedFile) flush() {
	if b.pending.Len() == 0 {
		return
	}
	b.f.Write(b.pending.Bytes())
	b.pending.Reset()
	b.lines = 0
}

// close flushes the remaining lines and closes the file.
func (b *bufferedFile) close() {
	b.flush()
	b.f.Close()
}

// Init configures debugging output. console is the master switch: when false,
// debugging is disabled and no log file is written even if logPath is set.
// When true, output goes to the console and, when logPath is not empty, to a
// new log file that replaces the previous run. The file batches bufferLines
// lines per disk write. Init is safe to call again.
func Init(enableConsole bool, logPath string, bufferLines int) error {
	mu.Lock()
	defer mu.Unlock()
	closeLocked()

	console.Store(enableConsole)
	if !enableConsole || logPath == "" {
		return nil
	}
	if bufferLines <= 0 {
		bufferLines = DefaultBufferLines
	}
	f, err := os.Create(logPath)
	if err != nil {
		return err
	}
	logFile = &bufferedFile{f: f, maxLines: bufferLines}
	return nil
}

// Log writes one structured debug line when debugging is enabled:
//
//	[DD-MM-YY@HH:mm:ss] [<module>] [<team>] : <msg>
//
// The timestamp is always stamped. module, team, and the message are each
// OPTIONAL: an empty module or team omits its bracket; an empty format omits the
// " : <msg>" tail (so Log("", "", "") is a line with only the timestamp). team is
// a separate bracket - NOT folded into the message - so one team's whole trace
// stays greppable (grep "[GRAY]" max-go.log).
func Log(module, team, format string, args ...any) {
	if !console.Load() {
		return // fast path with no lock: debugging off costs nothing (no lock, no format)
	}
	var b strings.Builder
	b.WriteString(time.Now().Format(timeLayout))
	// The module bracket, only when a tag was given.
	if module != "" {
		b.WriteString(" [")
		b.WriteString(module)
		b.WriteByte(']')
	}
	// The team bracket, only on a team scoped line.
	if team != "" {
		b.WriteString(" [")
		b.WriteString(team)
		b.WriteByte(']')
	}
	// The message tail, only when there is a message (else a line with only the timestamp).
	if format != "" {
		b.WriteString(" : ")
		b.WriteString(fmt.Sprintf(format, args...))
	}
	b.WriteByte('\n')
	emit(b.String())
}

// Logf is the common case helper for a line tagged with a module and no team:
// Logf(dbg.SFX, "looping %s", x) -> "[ts] [SFX] : looping ..." (it is
// Log(module, "", format, args...)).
func Logf(module, format string, args ...any) {
	Log(module, "", format, args...)
}

// Printf logs one untagged line through Log with no module or team:
// "[ts] : <msg>".
func Printf(format string, args ...any) {
	Log("", "", format, args...)
}

// emit writes one finished line to the console and the log file under the lock.
func emit(line string) {
	mu.Lock()
	defer mu.Unlock()
	// os.Stderr is read live (not captured) so output follows a console
	// reattach: ShowConsole rebinds os.Stderr to a freshly allocated console.
	os.Stderr.WriteString(line)
	if logFile != nil {
		logFile.write(line)
	}
}

// Flush forces any batched log file lines to disk (e.g. before an expected
// crash or when tailing the log while the game runs).
func Flush() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		logFile.flush()
	}
}

// Close flushes and closes the log file and disables output.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	closeLocked()
	console.Store(false)
}

// closeLocked closes the log file while mu is held.
func closeLocked() {
	if logFile != nil {
		logFile.close()
		logFile = nil
	}
}

// Enabled reports whether debugging output is on (the console master switch).
func Enabled() bool {
	return console.Load()
}
