package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestTruncatingWriterClipsLongLines(t *testing.T) {
	var buf bytes.Buffer
	tw := newTruncatingWriter(&buf, 10)
	if _, err := tw.Write([]byte("short\nthis line is way too long to fit\nfine\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !tw.Truncated() {
		t.Errorf("expected Truncated()=true")
	}
	want := "short\nthis li...\nfine\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTruncatingWriterUnTruncatedKeepsContent(t *testing.T) {
	var buf bytes.Buffer
	tw := newTruncatingWriter(&buf, 80)
	in := "hello\nworld\n"
	if _, err := tw.Write([]byte(in)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if tw.Truncated() {
		t.Errorf("Truncated()=true; want false")
	}
	if got := buf.String(); got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestTruncatingWriterFlushPartial(t *testing.T) {
	var buf bytes.Buffer
	tw := newTruncatingWriter(&buf, 10)
	if _, err := tw.Write([]byte("trailing-with-no-newline-here")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := buf.String(); got != "" {
		t.Errorf("buffer should be empty before Flush, got %q", got)
	}
	if err := tw.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	want := "trailin..."
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !tw.Truncated() {
		t.Errorf("expected Truncated()=true")
	}
}

func TestTruncatingWriterStreamedAcrossWrites(t *testing.T) {
	// Two writes, the line boundary lands inside the second write. Verifies
	// the buffer accumulates correctly across calls.
	var buf bytes.Buffer
	tw := newTruncatingWriter(&buf, 12)
	if _, err := tw.Write([]byte("partial line ")); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	if _, err := tw.Write([]byte("complete now\nnext\n")); err != nil {
		t.Fatalf("write 2: %v", err)
	}
	want := "partial l...\nnext\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestClipLineNarrowWidths(t *testing.T) {
	cases := []struct {
		line  string
		width int
		want  string
	}{
		{"abcdef", 0, ""},
		{"abcdef", 1, "a"},
		{"abcdef", 2, "ab"},
		{"abcdef", 3, "abc"},
		{"abcdef", 4, "a..."},
		{"abcdef", 5, "ab..."},
	}
	for _, c := range cases {
		got := string(clipLine([]byte(c.line), c.width))
		if got != c.want {
			t.Errorf("clipLine(%q, %d) = %q, want %q", c.line, c.width, got, c.want)
		}
	}
}

func TestTruncatingWriterTableHeader(t *testing.T) {
	// Realistic case: a wide table header gets clipped, hint should reflect
	// the actual width used.
	var buf bytes.Buffer
	tw := newTruncatingWriter(&buf, 20)
	header := "key                      key_capture_1  cpu  ram  status\n"
	if _, err := tw.Write([]byte(header)); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := strings.TrimSuffix(buf.String(), "\n")
	if len(got) != 20 {
		t.Errorf("clipped line length = %d, want 20", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("clipped line %q should end with ellipsis", got)
	}
}
