package main

import (
	"bytes"
	"io"
	"os"
	"syscall"
	"unsafe"
)

// isTerminal reports whether f points at a character device (a TTY rather
// than a pipe, file, or socket). It's used to decide whether long lines
// should be truncated to the terminal width.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// terminalWidth returns the column count of the controlling terminal for f.
// The second return is false if the lookup fails (non-TTY, or the ioctl
// errored), and the caller should treat that as "don't truncate".
func terminalWidth(f *os.File) (int, bool) {
	var ws struct {
		Row, Col       uint16
		XPixel, YPixel uint16
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TIOCGWINSZ,
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 || ws.Col == 0 {
		return 0, false
	}
	return int(ws.Col), true
}

// truncatingWriter wraps an io.Writer and clips each line to a fixed column
// width before forwarding it. Truncated lines end in "..." (within the same
// width budget) so the user can tell at a glance which rows were clipped.
//
// Bytes are buffered until a newline is seen, since truncation can only be
// applied one whole line at a time. Call Flush at the end of output to emit
// any trailing partial line.
//
// Truncation is byte-based, not rune- or grapheme-based — fine for the ASCII
// values qql tends to emit, and intentionally simple. Multi-byte characters
// landing on the cut boundary may render as one mojibake glyph, which is
// acceptable for an interactive convenience.
type truncatingWriter struct {
	w         io.Writer
	width     int
	buf       []byte
	truncated bool
}

func newTruncatingWriter(w io.Writer, width int) *truncatingWriter {
	return &truncatingWriter{w: w, width: width}
}

func (t *truncatingWriter) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	for {
		i := bytes.IndexByte(t.buf, '\n')
		if i < 0 {
			break
		}
		if err := t.emitLine(t.buf[:i], true); err != nil {
			return len(p), err
		}
		t.buf = t.buf[i+1:]
	}
	return len(p), nil
}

// Flush writes out any line that's still buffered without a trailing newline.
// Safe to call multiple times.
func (t *truncatingWriter) Flush() error {
	if len(t.buf) == 0 {
		return nil
	}
	err := t.emitLine(t.buf, false)
	t.buf = nil
	return err
}

func (t *truncatingWriter) Truncated() bool { return t.truncated }

func (t *truncatingWriter) emitLine(line []byte, withNewline bool) error {
	if len(line) > t.width {
		t.truncated = true
		line = clipLine(line, t.width)
	}
	if _, err := t.w.Write(line); err != nil {
		return err
	}
	if withNewline {
		if _, err := t.w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
}

// clipLine returns at most width bytes of line, with the final 3 chars
// replaced by "..." when there's room. width <= 0 yields an empty slice;
// width < 4 falls back to a hard byte cut without the ellipsis.
func clipLine(line []byte, width int) []byte {
	if width <= 0 {
		return nil
	}
	if width < 4 {
		out := make([]byte, width)
		copy(out, line)
		return out
	}
	out := make([]byte, width)
	copy(out, line[:width-3])
	out[width-3] = '.'
	out[width-2] = '.'
	out[width-1] = '.'
	return out
}
