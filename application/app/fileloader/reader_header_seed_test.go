package fileloader

import (
	"context"
	"testing"

	"breachline/app/interfaces"
)

// TestNewFileReader_SeedsHeaderFromTab verifies that a FileReader built from a
// FileTab carrying an open-time header returns that header without touching disk.
// The path is deliberately nonexistent: if Header() re-read from disk it would error.
func TestNewFileReader_SeedsHeaderFromTab(t *testing.T) {
	seeded := []string{"name", "timestamp"}
	tab := &interfaces.FileTab{
		FilePath: "/nonexistent/path/does-not-exist.csv",
		Headers:  seeded,
	}

	r := NewFileReader(tab, nil, context.Background())
	defer r.Close()

	got, err := r.Header()
	if err != nil {
		t.Fatalf("Header() returned error (header not seeded, re-read from disk): %v", err)
	}
	if len(got) != len(seeded) || got[0] != seeded[0] || got[1] != seeded[1] {
		t.Fatalf("Header() = %v, want %v", got, seeded)
	}
}

// TestNewFileReader_SeedsDirectoryHeader verifies that directory tabs also consume
// the tab's captured union header. Both the open path and the read path derive that
// header from the same cached directory snapshot, so they cannot diverge; without
// the seed, every query rebuilt a FileReader that re-resolved the schema of every
// member file in the directory.
func TestNewFileReader_SeedsDirectoryHeader(t *testing.T) {
	seeded := []string{"a", "b"}
	tab := &interfaces.FileTab{
		FilePath: "/nonexistent/dir",
		Headers:  seeded,
		Options:  interfaces.FileOptions{IsDirectory: true},
	}

	r := NewFileReader(tab, nil, context.Background())
	defer r.Close()

	got, err := r.Header()
	if err != nil {
		t.Fatalf("Header() returned error (header not seeded, re-read from disk): %v", err)
	}
	if len(got) != len(seeded) || got[0] != seeded[0] || got[1] != seeded[1] {
		t.Fatalf("Header() = %v, want %v", got, seeded)
	}
}

// TestNewFileReader_EmptyHeaderFallsBackToDisk verifies that a tab with no captured
// header does not seed the reader, so the disk-read fallback stays intact.
func TestNewFileReader_EmptyHeaderFallsBackToDisk(t *testing.T) {
	tab := &interfaces.FileTab{
		FilePath: "/nonexistent/path/does-not-exist.csv",
	}

	r := NewFileReader(tab, nil, context.Background())
	defer r.Close()

	if r.header != nil {
		t.Fatalf("reader header seeded from empty tab (%v), want nil", r.header)
	}
}
