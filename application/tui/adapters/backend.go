package adapters

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"breachline/app"
	"breachline/app/histogram"
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

	// Histogram data (from combined response)
	HistogramBuckets []histogram.HistogramBucket
	MinTs            int64
	MaxTs            int64
}

// SearchResult wraps a search response from the backend.
type SearchResult struct {
	Results    []interfaces.SearchResult
	TotalCount int
	Page       int
}

// LicenseInfo holds license details.
type LicenseInfo struct {
	Licensed bool
	Email    string
	ID       string
	Message  string
}

// Backend bridges the TUI frontend with the shared app backend.
type Backend struct {
	app       *app.App
	settings  *settings.SettingsService
	license   *app.LicenseService
	workspace *app.WorkspaceManager
	program   *tea.Program
}

// NewBackend creates a new backend adapter.
func NewBackend(appInstance *app.App, settingsService *settings.SettingsService, licenseService *app.LicenseService) *Backend {
	return &Backend{
		app:      appInstance,
		settings: settingsService,
		license:  licenseService,
	}
}

// SetWorkspace sets the workspace manager.
func (b *Backend) SetWorkspace(wm *app.WorkspaceManager) {
	b.workspace = wm
}

// SetProgram sets the Bubble Tea program reference for sending messages.
// It also registers the histogram-ready callback so async histogram results
// are delivered to the TUI program in headless/CLI mode.
func (b *Backend) SetProgram(p *tea.Program) {
	b.program = p
	b.app.SetHistogramReadyFunc(func(event interface{}) {
		if evt, ok := event.(*histogram.HistogramReadyEvent); ok && p != nil {
			if len(evt.Buckets) == 0 {
				return
			}
			var bucketMs int64
			if len(evt.Buckets) > 1 {
				bucketMs = evt.Buckets[1].Start - evt.Buckets[0].Start
			}
			viewBuckets := make([]HistogramBucket, len(evt.Buckets))
			for i, bkt := range evt.Buckets {
				viewBuckets[i] = HistogramBucket{
					TabID:     evt.TabID,
					StartTime: bkt.Start,
					EndTime:   bkt.Start + bucketMs,
					Count:     bkt.Count,
				}
			}
			p.Send(HistogramReadyMsg{TabID: evt.TabID, Buckets: viewBuckets})
		}
	})
}

// --- File Operations ---

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
		HistogramBuckets: resp.HistogramBuckets,
		MinTs:            resp.MinTs,
		MaxTs:            resp.MaxTs,
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

// GetTabs returns metadata for all open tabs.
func (b *Backend) GetTabs() []app.TabInfo {
	return b.app.GetTabs()
}

// --- Search ---

// SearchInFile performs a text search in a tab's data.
func (b *Backend) SearchInFile(tabID, term string, isRegex bool, page int, query string) (*SearchResult, error) {
	resp, err := b.app.SearchInFile(tabID, term, isRegex, page, query)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Results:    resp.Results,
		TotalCount: resp.TotalCount,
		Page:       resp.Page,
	}, nil
}

// CancelSearch cancels an in-progress search.
func (b *Backend) CancelSearch(tabID string) {
	b.app.CancelSearch(tabID)
}

// --- Annotations ---

// AddAnnotations adds annotations to selected rows.
func (b *Backend) AddAnnotations(req app.AnnotationSelectionRequest) (*app.AnnotationSelectionResult, error) {
	return b.app.AddAnnotationsToSelection(req)
}

// DeleteAnnotations removes annotations from selected rows.
func (b *Backend) DeleteAnnotations(req app.AnnotationSelectionRequest) (*app.AnnotationSelectionResult, error) {
	return b.app.DeleteAnnotationsFromSelection(req)
}

// GetFileAnnotations returns all annotations for a file.
func (b *Backend) GetFileAnnotations(fileHash string, opts interfaces.FileOptions) ([]*interfaces.FileAnnotationInfo, error) {
	return b.app.GetFileAnnotations(fileHash, opts)
}

// --- Clipboard ---

// CopySelection copies selected rows to clipboard.
func (b *Backend) CopySelection(req app.CopySelectionRequest) (*app.CopySelectionResult, error) {
	return b.app.CopySelectionToClipboard(req)
}

// --- Settings ---

// GetSettings returns the current application settings.
func (b *Backend) GetSettings() *interfaces.Settings {
	return b.app.GetEffectiveSettings()
}

// SaveSettings saves updated settings.
func (b *Backend) SaveSettings(s settings.Settings) error {
	return b.settings.SaveSettings(s)
}

// --- License ---

// IsLicensed returns whether the app has a valid license.
func (b *Backend) IsLicensed() bool {
	return app.IsLicensed()
}

// GetLicenseDetails returns license details.
func (b *Backend) GetLicenseDetails() (*LicenseInfo, error) {
	details, err := app.GetLicenseDetails()
	if err != nil {
		return &LicenseInfo{Licensed: false}, nil
	}
	return &LicenseInfo{
		Licensed: true,
		Email:    details.Email,
		ID:       details.ID,
		Message:  fmt.Sprintf("Licensed to %s (expires %s)", details.Email, details.EndDate.Format("2006-01-02")),
	}, nil
}

// ImportLicense imports a license file from a path.
func (b *Backend) ImportLicense(path string) error {
	content, err := readFileContent(path)
	if err != nil {
		return fmt.Errorf("read license file: %w", err)
	}
	return app.IsLicenseValid(strings.TrimSpace(content))
}

// --- Workspace ---

// IsWorkspaceOpen returns whether a workspace is open.
func (b *Backend) IsWorkspaceOpen() bool {
	if b.workspace == nil {
		return false
	}
	return b.workspace.IsWorkspaceOpen()
}

// GetWorkspaceName returns the name of the open workspace.
func (b *Backend) GetWorkspaceName() string {
	if b.workspace == nil {
		return ""
	}
	return b.workspace.GetWorkspaceName()
}

// GetWorkspaceFiles returns files in the workspace.
func (b *Backend) GetWorkspaceFiles() ([]*interfaces.WorkspaceFile, error) {
	if b.workspace == nil {
		return nil, fmt.Errorf("no workspace manager")
	}
	return b.workspace.GetWorkspaceFiles()
}

// OpenWorkspace opens a workspace file.
func (b *Backend) OpenWorkspace(path string) error {
	if b.workspace == nil {
		return fmt.Errorf("no workspace manager")
	}
	return b.workspace.OpenLocalWorkspace(path)
}

// CloseWorkspace closes the current workspace.
func (b *Backend) CloseWorkspace() error {
	if b.workspace == nil {
		return fmt.Errorf("no workspace manager")
	}
	return b.workspace.CloseWorkspace()
}

// --- CLIRuntimeAdapter ---

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
		r.program.Send(BackendEvent{Name: name, Data: data})
	}
}

// Log writes to stderr via the standard logger.
func (r *CLIRuntimeAdapter) Log(level string, message string) {
	log.Printf("[%s] %s", level, message)
}

// ShowOpenFileDialog is not supported in the CLI.
func (r *CLIRuntimeAdapter) ShowOpenFileDialog(opts interfaces.OpenDialogOptions) (string, error) {
	return "", fmt.Errorf("file dialogs not supported in CLI mode; provide file paths as arguments")
}

// ShowSaveFileDialog is not supported in the CLI.
func (r *CLIRuntimeAdapter) ShowSaveFileDialog(opts interfaces.SaveDialogOptions) (string, error) {
	return "", fmt.Errorf("save dialogs not supported in CLI mode")
}

// SetTitle sets the terminal window title using an escape sequence.
func (r *CLIRuntimeAdapter) SetTitle(title string) {
	fmt.Printf("\033]2;%s\007", title)
}

// InitBackend initialises the backend services without Wails.
func InitBackend() (*app.App, *settings.SettingsService, *app.LicenseService, *app.WorkspaceManager) {
	appInstance := app.NewApp()

	// Set CLI log function BEFORE Startup to avoid Wails runtime.EventsEmit panics.
	// The Wails runtime rejects context.Background() - this redirects all logging to stderr.
	appInstance.SetLogFunc(func(level, message string) {
		log.Printf("[%s] %s", level, message)
	})

	settingsService := settings.NewSettingsService()
	settingsService.SetCacheManager(appInstance)
	licenseService := app.NewLicenseService()
	licenseService.SetApp(appInstance)
	workspaceService := app.NewWorkspaceService()
	workspaceService.SetApp(appInstance)
	appInstance.SetWorkspaceService(workspaceService)

	ctx := context.Background()
	appInstance.Startup(ctx)
	settingsService.Startup(ctx)

	if err := settingsService.EnsureInstanceID(); err != nil {
		log.Printf("[warn] Failed to generate instance ID: %v", err)
	}

	licenseService.Startup(ctx)
	workspaceService.Startup(ctx)

	return appInstance, settingsService, licenseService, workspaceService
}

// BackendEvent is the tea.Msg sent by CLIRuntimeAdapter.EmitEvent.
type BackendEvent struct {
	Name string
	Data []interface{}
}

// LogMsg is a tea.Msg that delivers a backend log entry to the Console view.
type LogMsg struct {
	Level   string
	Message string
}

// HistogramBucket is a single time bucket delivered to the TUI histogram view.
type HistogramBucket struct {
	TabID     string
	StartTime int64
	EndTime   int64
	Count     int
}

// HistogramReadyMsg is sent to the Tea program when async histogram generation
// completes in headless/CLI mode (replaces the dropped EmitEvent path).
type HistogramReadyMsg struct {
	TabID   string
	Buckets []HistogramBucket
}

// readFileContent reads a file's content as a string.
func readFileContent(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
