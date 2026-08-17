//go:build darwin

package fileloader

import "golang.org/x/sys/unix"

// availableMemoryBytes returns an upper bound on usable memory, or 0 when it
// cannot be determined. Darwin has no direct "available" figure comparable to
// Linux's MemAvailable, so this reports installed physical memory; the caller's
// warning fraction keeps the resulting estimate conservative.
func availableMemoryBytes() int64 {
	total, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}
	return int64(total)
}
