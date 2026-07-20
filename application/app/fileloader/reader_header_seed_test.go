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

// TestNewFileReader_DoesNotSeedDirectoryHeader verifies that directory tabs do not
// consume the tab's captured header - the directory read path owns its own header.
func TestNewFileReader_DoesNotSeedDirectoryHeader(t *testing.T) {
	tab := &interfaces.FileTab{
		FilePath: "/nonexistent/dir",
		Headers:  []string{"a", "b"},
		Options:  interfaces.FileOptions{IsDirectory: true},
	}

	r := NewFileReader(tab, nil, context.Background())
	defer r.Close()

	if r.header != nil {
		t.Fatalf("directory tab header was seeded (%v), want nil", r.header)
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
