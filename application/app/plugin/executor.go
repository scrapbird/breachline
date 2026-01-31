package plugin

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	sharedtypes "github.com/scrapbird/breachline/shared/types"
)

// PluginMode represents the execution mode for a plugin
type PluginMode string

const (
	PluginModeHeader PluginMode = "header" // Get CSV header row
	PluginModeCount  PluginMode = "count"  // Get row count
	PluginModeStream PluginMode = "stream" // Get full CSV output
)

// PluginExecutor handles execution of a plugin binary
type PluginExecutor struct {
	plugin *PluginInfo
}

// NewPluginExecutor creates a new executor for the given plugin
func NewPluginExecutor(plugin *PluginInfo) *PluginExecutor {
	return &PluginExecutor{
		plugin: plugin,
	}
}

// Execute runs the plugin with the specified mode and returns raw output
func (e *PluginExecutor) Execute(ctx context.Context, mode PluginMode, filePath string, options sharedtypes.FileOptions) ([]byte, error) {
	// Build command
	cmd := exec.CommandContext(ctx, e.plugin.ExecPath,
		fmt.Sprintf("--mode=%s", mode),
		fmt.Sprintf("--file=%s", filePath),
	)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()

	// Check for errors
	if err != nil {
		// Include stderr in error message
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return nil, fmt.Errorf("plugin %s failed: %v\nstderr: %s", e.plugin.Config.Name, err, stderrStr)
		}
		return nil, fmt.Errorf("plugin %s failed: %v", e.plugin.Config.Name, err)
	}

	return stdout.Bytes(), nil
}

// ReadHeader executes the plugin in header mode and returns the CSV headers
func (e *PluginExecutor) ReadHeader(ctx context.Context, filePath string, options sharedtypes.FileOptions) ([]string, error) {
	// If NoHeaderRow is set, we can't use header mode as it might fail or return data as header
	// Instead, we use stream mode to get the first row and generate synthetic headers
	if options.NoHeaderRow {
		output, err := e.Execute(ctx, PluginModeStream, filePath, options)
		if err != nil {
			return nil, err
		}

		reader := csv.NewReader(bytes.NewReader(output))
		firstRow, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("plugin %s returned empty output", e.plugin.Config.Name)
			}
			return nil, fmt.Errorf("plugin %s returned invalid CSV: %v", e.plugin.Config.Name, err)
		}

		// Generate synthetic headers
		return normalizeHeaders(make([]string, len(firstRow))), nil
	}

	output, err := e.Execute(ctx, PluginModeHeader, filePath, options)
	if err != nil {
		return nil, err
	}

	// Parse CSV header line
	reader := csv.NewReader(bytes.NewReader(output))
	headers, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("plugin %s returned empty header", e.plugin.Config.Name)
		}
		return nil, fmt.Errorf("plugin %s returned invalid CSV header: %v", e.plugin.Config.Name, err)
	}

	return headers, nil
}

// excelColumnName converts a 0-based index to Excel-style column name.
func excelColumnName(index int) string {
	result := ""
	index++ // Convert to 1-based for the algorithm

	for index > 0 {
		index-- // Adjust for 0-based letter indexing
		result = string(rune('A'+index%26)) + result
		index /= 26
	}

	return result
}

// normalizeHeaders replaces empty headers with Excel-style column names
func normalizeHeaders(header []string) []string {
	normalized := make([]string, len(header))
	emptyCount := 0

	for i, h := range header {
		if strings.TrimSpace(h) == "" {
			normalized[i] = "Unnamed_" + excelColumnName(emptyCount)
			emptyCount++
		} else {
			normalized[i] = h
		}
	}

	return normalized
}

// GetRowCount executes the plugin in count mode and returns the row count
func (e *PluginExecutor) GetRowCount(ctx context.Context, filePath string, options sharedtypes.FileOptions) (int, error) {
	output, err := e.Execute(ctx, PluginModeCount, filePath, options)
	if err != nil {
		return 0, err
	}

	// Parse count (single integer)
	countStr := strings.TrimSpace(string(output))
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return 0, fmt.Errorf("plugin %s returned invalid count: %v (output: %s)", e.plugin.Config.Name, err, countStr)
	}

	if count < 0 {
		return 0, fmt.Errorf("plugin %s returned negative count: %d", e.plugin.Config.Name, count)
	}

	return count, nil
}

// GetReader executes the plugin in stream mode and returns a CSV reader
func (e *PluginExecutor) GetReader(ctx context.Context, filePath string, options sharedtypes.FileOptions) (*csv.Reader, io.ReadCloser, error) {
	output, err := e.Execute(ctx, PluginModeStream, filePath, options)
	if err != nil {
		return nil, nil, err
	}

	// Create a CSV reader from the output
	reader := csv.NewReader(bytes.NewReader(output))

	// Create a no-op closer since we're reading from memory
	closer := io.NopCloser(bytes.NewReader(output))

	return reader, closer, nil
}

// GetReaderWithoutCloser is a helper that returns just the CSV reader
// This is useful when the caller doesn't need to manage the closer
func (e *PluginExecutor) GetReaderWithoutCloser(ctx context.Context, filePath string, options sharedtypes.FileOptions) (*csv.Reader, error) {
	reader, _, err := e.GetReader(ctx, filePath, options)
	return reader, err
}
