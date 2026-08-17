//go:build windows

package fileloader

import (
	"syscall"
	"unsafe"
)

// memoryStatusEx mirrors the Win32 MEMORYSTATUSEX structure.
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatus = kernel32.NewProc("GlobalMemoryStatusEx")
)

// availableMemoryBytes returns physical memory currently available, or 0 when it
// cannot be determined.
func availableMemoryBytes() int64 {
	var status memoryStatusEx
	status.Length = uint32(unsafe.Sizeof(status))

	ret, _, _ := procGlobalMemoryStatus.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 {
		return 0
	}
	return int64(status.AvailPhys)
}
