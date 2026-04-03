package adapters

import (
	"context"
	"fmt"
	"log"

	"breachline/app"
	"breachline/app/interfaces"
	"breachline/app/settings"

	tea "github.com/charmbracelet/bubbletea"
)

// FileInfo holds metadata about an opened file tab.
type FileInfo struct {
	TabID    string
	FilePath string
	FileName string
	FileHash string
	Headers  []string
}

// RowsPage represents a page of data rows.
type RowsPage struct {
	Header           []string
	Rows             [][]string
	Total            int
	ReachedEnd       bool
	Annotations      []bool
	AnnotationColors []string
}

// Backend bridges the TUI frontend with the shared app backend.
// It calls Go functions directly (no IPC).
type Backend struct {
	app      *app.App
	settings *settings.SettingsService
	license  *app.LicenseService
	program  *tea.Program
}

// NewBackend creates a new backend adapter.
func NewBackend(appInstance *app.App, settingsService *settings.SettingsService, licenseService *app.LicenseService) *Backend {
	return &Backend{
		app:      appInstance,
		settings: settingsService,
		license:  licenseService,
	}
}

// SetProgram sets the Bubble Tea program reference for sending messages.
// Must be called after the program is created.
func (b *Backend) SetProgram(p *tea.Program) {
	b.program = p
}

// OpenFile opens a file and returns tab metadata.
func (b *Backend) OpenFile(path string, opts interfaces.FileOptions) (*FileInfo, error) {
	tabInfo, err := b.app.OpenFileTabWithOptions(path, opts)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return &FileInfo{
		TabID:    tabInfo.ID,
		FilePath: tabInfo.FilePath,
		FileName: tabInfo.FileName,
		FileHash: tabInfo.FileHash,
		Headers:  tabInfo.Headers,
	}, nil
}

// GetRows fetches a page of rows for a tab, optionally applying a query.
func (b *Backend) GetRows(tabID string, query string, startRow int, count int) (*RowsPage, error) {
	tab := b.app.GetTab(tabID)
	if tab == nil {
		return nil, fmt.Errorf("tab not found: %s", tabID)
	}

	// Detect timestamp field for sorting
	timeField := ""
	if tab.SortedTimeField != "" {
		timeField = tab.SortedTimeField
	}

	resp, err := b.app.GetDataAndHistogram(tabID, startRow, startRow+count, query, timeField, 0)
	if err != nil {
		return nil, fmt.Errorf("get rows: %w", err)
	}

	return &RowsPage{
		Header:           resp.Header,
		Rows:             resp.Rows,
		Total:            resp.Total,
		ReachedEnd:       resp.ReachedEnd,
		Annotations:      resp.Annotations,
		AnnotationColors: resp.AnnotationColors,
	}, nil
}

// GetRowCount returns the total row count for a tab.
func (b *Backend) GetRowCount(tabID string) (int, error) {
	tab := b.app.GetTab(tabID)
	if tab == nil {
		return 0, fmt.Errorf("tab not found: %s", tabID)
	}

	timeField := ""
	if tab.SortedTimeField != "" {
		timeField = tab.SortedTimeField
	}

	resp, err := b.app.GetDataAndHistogram(tabID, 0, 0, "", timeField, 0)
	if err != nil {
		return 0, fmt.Errorf("get row count: %w", err)
	}
	return resp.Total, nil
}

// CloseTab closes a file tab.
func (b *Backend) CloseTab(tabID string) error {
	return b.app.CloseTab(tabID)
}

// GetSettings returns the current application settings.
func (b *Backend) GetSettings() *interfaces.Settings {
	return b.app.GetEffectiveSettings()
}

// CLIRuntimeAdapter implements interfaces.RuntimeAdapter for the CLI/TUI environment.
type CLIRuntimeAdapter struct {
	program *tea.Program
}

// NewCLIRuntimeAdapter creates a runtime adapter that bridges to Bubble Tea.
func NewCLIRuntimeAdapter() *CLIRuntimeAdapter {
	return &CLIRuntimeAdapter{}
}

// SetProgram sets the Bubble Tea program for event delivery.
func (r *CLIRuntimeAdapter) SetProgram(p *tea.Program) {
	r.program = p
}

// EmitEvent converts a backend event into a Bubble Tea message.
func (r *CLIRuntimeAdapter) EmitEvent(name string, data ...interface{}) {
	if r.program != nil {
		r.program.Send(backendEvent{Name: name, Data: data})
	}
}

// Log writes to stderr via the standard logger.
func (r *CLIRuntimeAdapter) Log(level string, message string) {
	log.Printf("[%s] %s", level, message)
}

// ShowOpenFileDialog is not supported in the CLI; callers should use paths directly.
func (r *CLIRuntimeAdapter) ShowOpenFileDialog(opts interfaces.OpenDialogOptions) (string, error) {
	return "", fmt.Errorf("file dialogs not supported in CLI mode; provide file paths as arguments")
}

// ShowSaveFileDialog is not supported in the CLI.
func (r *CLIRuntimeAdapter) ShowSaveFileDialog(opts interfaces.SaveDialogOptions) (string, error) {
	return "", fmt.Errorf("save dialogs not supported in CLI mode")
}

// SetTitle sets the terminal window title using an escape sequence.
func (r *CLIRuntimeAdapter) SetTitle(title string) {
	// OSC 2 escape sequence for terminal title
	fmt.Printf("\033]2;%s\007", title)
}

// InitBackend initialises the backend services without Wails.
// It creates a background context for the app lifecycle.
func InitBackend() (*app.App, *settings.SettingsService, *app.LicenseService) {
	appInstance := app.NewApp()
	settingsService := settings.NewSettingsService()
	settingsService.SetCacheManager(appInstance)
	licenseService := app.NewLicenseService()
	licenseService.SetApp(appInstance)

	// Create a background context (no Wails)
	ctx := context.Background()
	appInstance.Startup(ctx)
	settingsService.Startup(ctx)

	if err := settingsService.EnsureInstanceID(); err != nil {
		log.Printf("[warn] Failed to generate instance ID: %v", err)
	}

	licenseService.Startup(ctx)

	return appInstance, settingsService, licenseService
}

// backendEvent is the tea.Msg sent by CLIRuntimeAdapter.EmitEvent.
type backendEvent struct {
	Name string
	Data []interface{}
}
