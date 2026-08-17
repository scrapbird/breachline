package app

import (
	"breachline/app/fileloader"
	"breachline/app/interfaces"
	"breachline/app/settings"
	"breachline/app/timestamps"
	"context"
	"encoding/csv"
	"fmt"
	"os"
)

// Internal helper methods that operate on FileTab instances

// directoryLoadOptions builds the loader options for a directory tab.
//
// The ingest timezone is resolved into the options rather than left blank. It forms
// part of the directory snapshot's cache key, because resolving a member's columns
// parses it and parsed rows are cached per timezone. The reader fills the timezone
// in when it loads rows, so any caller that left it blank stored or looked up a
// snapshot under a different key than the reader used, and the first query rescanned
// the whole directory: on a 145,780 file archive an extra 0.8s and a second full
// stat of every file, on top of re-resolving the sampled members' columns.
func directoryLoadOptions(opts interfaces.FileOptions) fileloader.FileOptions {
	loadOptions := fileloader.FileOptions{
		JPath:                  opts.JPath,
		NoHeaderRow:            opts.NoHeaderRow,
		IncludeSourceColumn:    opts.IncludeSourceColumn,
		IngestTimezoneOverride: opts.IngestTimezoneOverride,
		FilePattern:            opts.FilePattern,
	}
	if loadOptions.IngestTimezoneOverride == "" {
		loadOptions.IngestTimezoneOverride = timestamps.GetIngestTimezoneWithOverride(opts.IngestTimezoneOverride).String()
	}
	return loadOptions
}

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

	// Handle directory tabs. These address the shared snapshot, so they need the
	// timezone resolved into the options; a plain file takes it as an argument.
	if tab.Options.IsDirectory {
		return fileloader.ReadHeaderForPath(tab.FilePath, directoryLoadOptions(tab.Options), ingestTz)
	}

	// Use proxy function that handles all file types with options
	return fileloader.ReadHeaderWithOptions(tab.FilePath, options, ingestTz)
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
	maxFiles := currentSettings.DirectoryFileLimit()

	options := directoryLoadOptions(tab.Options)

	// Reuse the snapshot captured when the tab was opened, so this does not rescan
	// the directory and re-resolve every member's schema.
	info, err := fileloader.GetDirectorySnapshot(context.Background(), tab.FilePath, options, maxFiles, nil)
	if err != nil {
		return nil, err
	}

	// Create reader
	return fileloader.NewDirectoryReader(info, options)
}

// materializeQueryRowsForTab computes and returns the full set of rows that match the provided query
// for a specific tab, honoring the current settings and leveraging the tab's query cache
func (a *App) materializeQueryRowsForTab(tab *FileTab, query string, timeField string) ([]string, [][]string, error) {
	if tab == nil || tab.FilePath == "" {
		return nil, [][]string{}, nil
	}

	return a.ExecuteQueryForTab(tab, query, timeField)
}
