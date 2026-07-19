package fileloader

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// makeFiles creates n empty files named 000.log, 001.log, ... in dir.
func makeFiles(t *testing.T, dir string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%03d.log", i))
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
}

func TestDiscoverFiles_TruncationAndUnlimited(t *testing.T) {
	dir := t.TempDir()
	makeFiles(t, dir, 25)

	cases := []struct {
		name          string
		maxFiles      int
		wantCount     int
		wantTruncated bool
	}{
		{"cap below total truncates", 10, 10, true},
		{"cap equal to total does not truncate", 25, 25, false},
		{"cap above total does not truncate", 100, 25, false},
		{"zero means unlimited", 0, 25, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := DiscoverFiles(dir, DirectoryDiscoveryOptions{
				Pattern:  "*.log",
				MaxFiles: tc.maxFiles,
			}, nil)
			if err != nil {
				t.Fatalf("DiscoverFiles: %v", err)
			}
			if len(info.Files) != tc.wantCount {
				t.Errorf("files loaded = %d, want %d", len(info.Files), tc.wantCount)
			}
			if info.Truncated != tc.wantTruncated {
				t.Errorf("truncated = %v, want %v", info.Truncated, tc.wantTruncated)
			}
		})
	}
}
