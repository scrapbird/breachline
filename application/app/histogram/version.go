package histogram

import (
	"fmt"
	"sync/atomic"
)


// VersionManager interface for managing histogram versions
// This allows the histogram package to work with FileTab without importing app package
type VersionManager interface {
	GetHistogramVersion() int64
	IncrementHistogramVersion() int64
}


// GetVersion returns the current histogram version for a tab
func GetVersion(tabID string, versionNum int64) string {
	return fmt.Sprintf("%s:%d", tabID, versionNum)
}


// IncrementVersion increments an atomic version counter and returns the new version string
func IncrementVersion(tabID string, versionCounter *int64) string {
	newVersion := atomic.AddInt64(versionCounter, 1)
	return fmt.Sprintf("%s:%d", tabID, newVersion)
}


// LoadVersion loads the current version from an atomic counter
func LoadVersion(tabID string, versionCounter *int64) string {
	version := atomic.LoadInt64(versionCounter)
	return fmt.Sprintf("%s:%d", tabID, version)
}
