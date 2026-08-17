package fileloader

import (
	"fmt"
	"os"
	"time"

	"breachline/app/timestamps"
)

// Directory loads are fully materialized in memory: every row of every member file
// ends up resident as a []string. For a compressed archive the on-disk size says
// almost nothing about that footprint, so a 5 GB directory of gzipped JSON can ask
// for tens of gigabytes and take the app down with it. The estimate here samples a
// few members to measure the real compression ratio, projects the resident size,
// and lets the UI warn before the user commits to a load that cannot finish.

// memoryEstimateSampleFiles is how many members are decompressed to measure the
// compression ratio. Spread across discovery order, a handful is enough: members
// of one archive compress almost identically.
const memoryEstimateSampleFiles = 5

// residentBytesPerDataByte projects resident memory from uncompressed data size.
// Measured at roughly 1.1x for the row strings themselves; the rest covers the Row
// wrappers, the parse peak and the base-data cache entry.
const residentBytesPerDataByte = 2.0

// memoryWarnFraction is the share of available memory a projected load may occupy
// before the user is warned.
const memoryWarnFraction = 0.7

// availableMemory reports usable memory. Indirected through a variable so tests can
// exercise the warning threshold without depending on the host's actual memory.
var availableMemory = availableMemoryBytes

// DirectoryMemoryEstimate projects what loading a directory will cost in memory.
type DirectoryMemoryEstimate struct {
	// CompressionRatio is uncompressed bytes per on-disk byte (1.0 if uncompressed).
	CompressionRatio float64
	// EstimatedBytes is the projected uncompressed size of the whole dataset.
	EstimatedBytes int64
	// EstimatedResidentBytes is the projected peak in-memory footprint.
	EstimatedResidentBytes int64
	// AvailableBytes is what the machine reports as available, or 0 if unknown.
	AvailableBytes int64
	// Warning is set when the projection exceeds what the machine can hold.
	Warning string
}

// EstimateDirectoryMemory projects the memory cost of loading a discovered
// directory. It samples a few members to measure the true compression ratio.
func EstimateDirectoryMemory(info *DirectoryInfo) DirectoryMemoryEstimate {
	estimate := DirectoryMemoryEstimate{CompressionRatio: 1.0}
	if info == nil || len(info.Files) == 0 {
		return estimate
	}

	estimate.CompressionRatio = sampleCompressionRatio(info)
	estimate.EstimatedBytes = int64(float64(info.TotalSize) * estimate.CompressionRatio)
	estimate.EstimatedResidentBytes = int64(float64(estimate.EstimatedBytes) * residentBytesPerDataByte)
	estimate.AvailableBytes = availableMemory()

	if estimate.AvailableBytes > 0 &&
		float64(estimate.EstimatedResidentBytes) > float64(estimate.AvailableBytes)*memoryWarnFraction {
		estimate.Warning = fmt.Sprintf(
			"This directory is about %s of data (%s on disk, expanding %.0fx) and loading it is projected to need around %s of memory, but only %s is available. The load may exhaust memory before it completes.",
			formatBytes(estimate.EstimatedBytes),
			formatBytes(info.TotalSize),
			estimate.CompressionRatio,
			formatBytes(estimate.EstimatedResidentBytes),
			formatBytes(estimate.AvailableBytes),
		)
	}

	return estimate
}

// sampleCompressionRatio decompresses a spread of members and returns the measured
// uncompressed-to-on-disk ratio. Falls back to 1.0 when nothing can be sampled.
func sampleCompressionRatio(info *DirectoryInfo) float64 {
	indexes := schemaSampleIndexes(len(info.Files), memoryEstimateSampleFiles)

	var onDisk, uncompressed int64
	for _, idx := range indexes {
		filePath := info.Files[idx]

		_, compression := detectFileTypeAndCompressionCached(filePath)

		stat, ok := info.Stats[filePath]
		if !ok {
			fi, err := os.Stat(filePath)
			if err != nil {
				continue
			}
			stat = DirectoryFileStat{Size: fi.Size(), ModTime: fi.ModTime().UnixNano()}
		}
		if stat.Size == 0 {
			continue
		}

		if compression == CompressionNone {
			onDisk += stat.Size
			uncompressed += stat.Size
			continue
		}

		result, err := DecompressFile(filePath, compression)
		if err != nil {
			continue
		}
		onDisk += stat.Size
		uncompressed += int64(len(result.Data))
	}

	if onDisk == 0 || uncompressed == 0 {
		return 1.0
	}
	return float64(uncompressed) / float64(onDisk)
}

// formatBytes renders a byte count for display in warnings.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTP"[exp])
}

// effectiveIngestTimezone resolves the ingest timezone for a per-file override,
// falling back to the configured default.
func effectiveIngestTimezone(override string) *time.Location {
	return timestamps.GetIngestTimezoneWithOverride(override)
}
