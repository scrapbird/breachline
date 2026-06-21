package interfaces

// RuntimeAdapter abstracts the runtime environment (Wails desktop vs CLI terminal).
// The desktop build provides a Wails implementation; the CLI build provides a
// Bubble Tea implementation that converts events to tea.Msg values.
type RuntimeAdapter interface {
	// EmitEvent sends a named event with optional data to the frontend.
	// In Wails this calls runtime.EventsEmit; in the TUI it sends a tea.Msg.
	EmitEvent(name string, data ...interface{})

	// Log writes a structured log message.
	// level is one of "debug", "info", "warn", "error".
	Log(level string, message string)

	// ShowOpenFileDialog opens a file picker and returns the selected path.
	// Returns empty string if the user cancels.
	ShowOpenFileDialog(opts OpenDialogOptions) (string, error)

	// ShowSaveFileDialog opens a save dialog and returns the chosen path.
	// Returns empty string if the user cancels.
	ShowSaveFileDialog(opts SaveDialogOptions) (string, error)

	// SetTitle updates the application title (window title or terminal title).
	SetTitle(title string)
}

// OpenDialogOptions mirrors the options for an open-file dialog.
type OpenDialogOptions struct {
	Title           string
	DefaultFilename string
	Filters         []FileFilter
}

// SaveDialogOptions mirrors the options for a save-file dialog.
type SaveDialogOptions struct {
	Title           string
	DefaultFilename string
	Filters         []FileFilter
}

// FileFilter describes a file type filter for dialog boxes.
type FileFilter struct {
	DisplayName string
	Pattern     string
}
