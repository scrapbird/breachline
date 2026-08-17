//go:build !linux && !darwin && !windows

package fileloader

// availableMemoryBytes has no implementation on this platform. Returning 0 means
// "unknown", which suppresses the memory warning rather than guessing at it.
func availableMemoryBytes() int64 { return 0 }
