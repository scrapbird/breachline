package fileloader

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// withAvailableMemory pins the reported available memory for a test.
func withAvailableMemory(t *testing.T, bytes int64) {
	t.Helper()
	previous := availableMemory
	availableMemory = func() int64 { return bytes }
	t.Cleanup(func() { availableMemory = previous })
}

// TestEstimateDirectoryMemory_MeasuresCompressionRatio verifies the projection is
// based on a measured decompression ratio rather than on-disk size. For a gzipped
// archive the two differ by an order of magnitude, which is exactly why a user can
// open what looks like a 5 GB directory and exhaust far more memory than that.
func TestEstimateDirectoryMemory_MeasuresCompressionRatio(t *testing.T) {
	resetDirectoryCaches(t)
	withAvailableMemory(t, 64<<30) // Plenty: no warning expected.

	dir := t.TempDir()
	// Highly compressible content, so the on-disk size badly understates the data.
	body := `{"Records":[{"id":"` + strings.Repeat("a", 20000) + `"}]}`
	for i := 0; i < 4; i++ {
		writeGzipJSON(t, filepath.Join(dir, string(rune('a'+i))+".json.gz"), body)
	}

	info, err := DiscoverFiles(dir, DirectoryDiscoveryOptions{Pattern: "*.json.gz"}, nil)
	if err != nil {
		t.Fatalf("DiscoverFiles: %v", err)
	}

	estimate := EstimateDirectoryMemory(info)

	if estimate.CompressionRatio <= 1.0 {
		t.Fatalf("compression ratio = %.2f, want > 1 for gzipped members", estimate.CompressionRatio)
	}
	if estimate.EstimatedBytes <= info.TotalSize {
		t.Fatalf("estimated size %d must exceed on-disk size %d for compressed members",
			estimate.EstimatedBytes, info.TotalSize)
	}
	if estimate.Warning != "" {
		t.Fatalf("unexpected warning with ample memory available: %s", estimate.Warning)
	}
}

// TestEstimateDirectoryMemory_WarnsWhenOverAvailable verifies the user is warned
// before committing to a load projected to need more memory than the machine has.
func TestEstimateDirectoryMemory_WarnsWhenOverAvailable(t *testing.T) {
	resetDirectoryCaches(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.csv"), "id,name\n1,alice\n")

	info, err := DiscoverFiles(dir, DirectoryDiscoveryOptions{Pattern: "*.csv"}, nil)
	if err != nil {
		t.Fatalf("DiscoverFiles: %v", err)
	}

	// One byte of available memory: any dataset at all exceeds the threshold.
	withAvailableMemory(t, 1)

	estimate := EstimateDirectoryMemory(info)
	if estimate.Warning == "" {
		t.Fatal("no warning when the projected load exceeds available memory")
	}
}

// TestEstimateDirectoryMemory_NoWarningWhenMemoryUnknown verifies that a platform
// which cannot report available memory stays silent rather than guessing.
func TestEstimateDirectoryMemory_NoWarningWhenMemoryUnknown(t *testing.T) {
	resetDirectoryCaches(t)
	withAvailableMemory(t, 0)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.csv"), "id,name\n1,alice\n")

	info, err := DiscoverFiles(dir, DirectoryDiscoveryOptions{Pattern: "*.csv"}, nil)
	if err != nil {
		t.Fatalf("DiscoverFiles: %v", err)
	}

	if estimate := EstimateDirectoryMemory(info); estimate.Warning != "" {
		t.Fatalf("warned despite unknown available memory: %s", estimate.Warning)
	}
}

// TestPreviewDirectory_ReportsEstimateAndSampling verifies the preview carries the
// projection and the sampling note through to the frontend.
func TestPreviewDirectory_ReportsEstimateAndSampling(t *testing.T) {
	resetDirectoryCaches(t)
	withSchemaSample(t, 2)
	withAvailableMemory(t, 64<<30)

	dir := t.TempDir()
	makeJSONArchive(t, dir, 10, -1)

	result, err := PreviewDirectoryContext(context.Background(), dir, "*.json.gz", "$.Records", 0, nil)
	if err != nil {
		t.Fatalf("PreviewDirectoryContext: %v", err)
	}

	if result.TotalFiles != 10 {
		t.Fatalf("totalFiles = %d, want 10", result.TotalFiles)
	}
	if !result.SampledSchema {
		t.Fatal("preview did not report that the schema was sampled")
	}
	if result.SchemaSampled != 2 {
		t.Fatalf("schemaSampled = %d, want 2", result.SchemaSampled)
	}
	if result.EstimatedUncompressedSize <= 0 {
		t.Fatal("preview reported no size estimate")
	}
}
