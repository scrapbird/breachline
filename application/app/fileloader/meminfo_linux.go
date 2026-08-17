//go:build linux

package fileloader

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// availableMemoryBytes returns memory the machine can hand out without swapping,
// or 0 when it cannot be determined. MemAvailable is the kernel's own estimate and
// accounts for reclaimable page cache, so it is a better bound than MemFree.
func availableMemoryBytes() int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "MemAvailable:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return kb * 1024
	}

	return 0
}
