package app

import (
	"breachline/app/fileloader"
	"breachline/app/settings"
	"breachline/app/timestamps"
	"encoding/csv"
	"fmt"
	"os"
)

// Internal helper methods that operate on FileTab instances

// readHeaderForTab reads the header for a specific tab (supports CSV, XLSX, JSON, and directories)
// Uses the tab's NoHeaderRow setting to determine how to parse headers
func (a *App) readHeaderForTab(tab *FileTab) ([]string, error) {
	if tab == nil || tab.FilePath == "" {
		return nil, fmt.Errorf("no file opened in tab")
	}

	// Build unified file options from tab settings
	options := fileloader.FileOptions{
		NoHeaderRow:         tab.Options.NoHeaderRow,
		JPath:               tab.Options.JPath,
		IncludeSourceColumn: tab.Options.IncludeSourceColumn,
		FilePattern:         tab.Options.FilePattern,
	}

	// Get effective ingest timezone - IMPORTANT: pass this to ensure consistent cache keys
	ingestTz := timestamps.GetIngestTimezoneWithOverride(tab.Options.IngestTimezoneOverride)

	// Handle directory tabs
	if tab.Options.IsDirectory {
		return fileloader.ReadHeaderForPath(tab.FilePath, options, ingestTz)
	}

	// Use proxy function that handles all file types with options
	return fileloader.ReadHeaderWithOptions(tab.FilePath, options, ingestTz)
}

// getRowCountForTab returns the total number of data rows for a specific tab (supports CSV, XLSX, JSON, and directories)
func (a *App) getRowCountForTab(tab *FileTab) (int, error) {
	if tab == nil || tab.FilePath == "" {
		return 0, nil
	}

	// Pass full tab options including IngestTimezoneOverride for consistent cache keys
	options := tab.Options

	// Handle directory tabs
	if tab.Options.IsDirectory {
		return fileloader.GetRowCountForPath(tab.FilePath, options)
	}

	// Use proxy function that handles all file types with options
	// IMPORTANT: Pass full options including IngestTimezoneOverride for consistent cache keys
	return fileloader.GetRowCountWithOptions(tab.FilePath, options)
}

// getReaderForTab returns a reader for the tab's file (supports CSV, XLSX, JSON, and directories)
// For directories, returns a DirectoryReader that iterates through all files
func (a *App) getReaderForTab(tab *FileTab) (*csv.Reader, *os.File, error) {
	if tab == nil || tab.FilePath == "" {
		return nil, nil, fmt.Errorf("no file opened in tab")
	}

	// Handle directory tabs - return nil for csv.Reader, caller should use getDirectoryReaderForTab
	if tab.Options.IsDirectory {
		return nil, nil, fmt.Errorf("use getDirectoryReaderForTab for directory tabs")
	}

	// Use proxy function that handles all file types
	return fileloader.GetReader(tab.FilePath, tab.Options)
}

// getDirectoryReaderForTab returns a DirectoryReader for directory tabs
func (a *App) getDirectoryReaderForTab(tab *FileTab) (*fileloader.DirectoryReader, error) {
	if tab == nil || tab.FilePath == "" {
		return nil, fmt.Errorf("no file opened in tab")
	}

	if !tab.Options.IsDirectory {
		return nil, fmt.Errorf("tab is not a directory")
	}

	// Get max files setting
	currentSettings := settings.GetEffectiveSettings()
	maxFiles := currentSettings.MaxDirectoryFiles
	if maxFiles <= 0 {
		maxFiles = 500 // Default
	}

	// Discover files
	info, err := fileloader.DiscoverFiles(tab.FilePath, fileloader.DirectoryDiscoveryOptions{
		Pattern:  tab.Options.FilePattern,
		MaxFiles: maxFiles,
	}, nil)
	if err != nil {
		return nil, err
	}

	// Create reader
	return fileloader.NewDirectoryReader(info, fileloader.FileOptions{
		JPath:                  tab.Options.JPath,
		NoHeaderRow:            tab.Options.NoHeaderRow,
		IncludeSourceColumn:    tab.Options.IncludeSourceColumn,
		IngestTimezoneOverride: tab.Options.IngestTimezoneOverride,
	})
}

// materializeQueryRowsForTab computes and returns the full set of rows that match the provided query
// for a specific tab, honoring the current settings and leveraging the tab's query cache
func (a *App) materializeQueryRowsForTab(tab *FileTab, query string, timeField string) ([]string, [][]string, error) {
	if tab == nil || tab.FilePath == "" {
		return nil, [][]string{}, nil
	}

	return a.ExecuteQueryForTab(tab, query, timeField)
}
