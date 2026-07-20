package fileloader

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// writeTempFile writes content to a new temp file and returns its path.
func writeTempFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestDetectFileTypeAndCompressionCached_HitMatchesUncached(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "data.csv", []byte("a,b,c\n1,2,3\n"))
	ClearDetectionCache(path)

	wantType, wantComp := DetectFileTypeAndCompression(path)

	// First call populates the cache, second call hits it. Both must match uncached.
	for i := 0; i < 2; i++ {
		gotType, gotComp := detectFileTypeAndCompressionCached(path)
		if gotType != wantType || gotComp != wantComp {
			t.Fatalf("call %d: got (%v, %v), want (%v, %v)", i, gotType, gotComp, wantType, wantComp)
		}
	}
}

func TestDetectFileTypeAndCompressionCached_GzipWithoutExtension(t *testing.T) {
	dir := t.TempDir()

	// A gzip stream in a file named .csv must still be detected as compressed
	// via the magic-byte peek on the (first) cache miss.
	path := filepath.Join(dir, "data.csv")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	gw := gzip.NewWriter(f)
	if _, err := gw.Write([]byte("a,b,c\n1,2,3\n")); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	gw.Close()
	f.Close()
	ClearDetectionCache(path)

	gotType, gotComp := detectFileTypeAndCompressionCached(path)
	if gotComp != CompressionGzip {
		t.Fatalf("expected gzip detected via magic bytes, got compression %v", gotComp)
	}
	// Magic-detected compressed files report inner type as CSV.
	if gotType != FileTypeCSV {
		t.Fatalf("expected FileTypeCSV inner type, got %v", gotType)
	}

	// Cached repeat returns the same pair.
	gotType2, gotComp2 := detectFileTypeAndCompressionCached(path)
	if gotType2 != gotType || gotComp2 != gotComp {
		t.Fatalf("cached repeat mismatch: got (%v, %v), want (%v, %v)", gotType2, gotComp2, gotType, gotComp)
	}
}

func TestDetectFileTypeAndCompressionCached_Invalidation(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "data.csv", []byte("a,b,c\n1,2,3\n"))
	ClearDetectionCache(path)

	// Prime the cache as a plain CSV.
	_, comp := detectFileTypeAndCompressionCached(path)
	if comp != CompressionNone {
		t.Fatalf("expected uncompressed CSV, got %v", comp)
	}

	// Overwrite the same path with a gzip stream (different size => guard mismatch),
	// forcing re-detection.
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	gw := gzip.NewWriter(f)
	if _, err := gw.Write([]byte("x,y,z\n4,5,6\n")); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	gw.Close()
	f.Close()

	_, comp2 := detectFileTypeAndCompressionCached(path)
	if comp2 != CompressionGzip {
		t.Fatalf("expected re-detection to find gzip after file change, got %v", comp2)
	}
}

func TestClearDetectionCache(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "data.csv", []byte("a,b\n1,2\n"))

	detectFileTypeAndCompressionCached(path)
	absPath, _ := filepath.Abs(path)

	detectionCacheMu.RLock()
	_, found := detectionCache[absPath]
	detectionCacheMu.RUnlock()
	if !found {
		t.Fatalf("expected cache entry after detection")
	}

	ClearDetectionCache(path)

	detectionCacheMu.RLock()
	_, found = detectionCache[absPath]
	detectionCacheMu.RUnlock()
	if found {
		t.Fatalf("expected cache entry removed after ClearDetectionCache")
	}
}
