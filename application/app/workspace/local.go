package workspace

// todo: make sure get file functions return annotations with them

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"breachline/app/fileloader"
	"breachline/app/interfaces"
	"breachline/app/settings"
	"breachline/app/timestamps"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
)

// annotationLocation tracks where an annotation is stored for O(1) lookup by ID
type annotationLocation struct {
	compositeKey string // fileHash::JPATH::jpath
	rowIndex     int    // original row index of the annotation
}

type LocalWorkspaceService struct {
	ctx            context.Context
	app            interfaces.AppService
	workspacePath  string
	workspaceMu    sync.RWMutex
	annotationsMap map[string]map[int]*interfaces.RowAnnotation // composite_key -> row_index -> annotation
	fileData       map[string]*interfaces.WorkspaceFile         // composite_key -> file data (hash, jpath, description)
	hashKey        []byte                                       // HighwayHash key (32 bytes)

	// Index by annotation ID for O(1) deletion
	annotationsByID map[string]*annotationLocation // annotationID -> location info

	// Cache invalidation
	cacheInvalidators []CacheInvalidator
	invalidatorMu     sync.RWMutex
}

// WorkspaceConfig represents the structure of a .breachline workspace file
type WorkspaceConfig struct {
	HashKey string                     `yaml:"hash_key"` // Base64-encoded 32-byte key for HighwayHash
	Files   []interfaces.WorkspaceFile `yaml:"files"`
}

// NewLocalWorkspaceService creates a new local workspace service
func NewLocalWorkspaceService() *LocalWorkspaceService {
	return &LocalWorkspaceService{
		annotationsMap:  make(map[string]map[int]*interfaces.RowAnnotation),
		fileData:        make(map[string]*interfaces.WorkspaceFile),
		annotationsByID: make(map[string]*annotationLocation),
		hashKey:         nil,
	}
}

// SetApp allows the main function to inject the App reference
func (lws *LocalWorkspaceService) SetApp(app interfaces.AppService) {
	lws.app = app
}

// Startup receives the Wails context
func (lws *LocalWorkspaceService) Startup(ctx context.Context) {
	lws.ctx = ctx
}

// CreateWorkspace creates a new workspace file at the specified path
func (lws *LocalWorkspaceService) CreateWorkspace(workspaceIdentifier string) error {
	if lws.ctx == nil {
		return errors.New("service not initialized")
	}

	// Check if licensed
	if !lws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	// Create empty workspace config
	workspace := WorkspaceConfig{
		Files: make([]interfaces.WorkspaceFile, 0),
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&workspace)
	if err != nil {
		return fmt.Errorf("failed to marshal workspace: %w", err)
	}

	// Write to file
	if err := os.WriteFile(workspaceIdentifier, data, 0644); err != nil {
		return fmt.Errorf("failed to write workspace file: %w", err)
	}

	// Load the newly created workspace
	if err := lws.OpenWorkspace(workspaceIdentifier); err != nil {
		return fmt.Errorf("failed to open created workspace: %w", err)
	}

	lws.app.Log("info", fmt.Sprintf("Workspace created: %s", workspaceIdentifier))
	return nil
}

// ChooseAndOpenWorkspace opens a workspace file dialog and loads annotations
func (lws *LocalWorkspaceService) ChooseAndOpenWorkspace() error {
	if lws.ctx == nil {
		return errors.New("service not initialized")
	}

	// Check if licensed
	if !lws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	// Open file dialog
	filePath, err := runtime.OpenFileDialog(lws.ctx, runtime.OpenDialogOptions{
		Title: "Open Workspace File",
		Filters: []runtime.FileFilter{
			{DisplayName: "BreachLine Workspace", Pattern: "*.breachline"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to open file dialog: %w", err)
	}

	// User cancelled
	if filePath == "" {
		return nil
	}

	return lws.OpenWorkspace(filePath)
}

// OpenWorkspace opens a workspace file and loads annotations
func (lws *LocalWorkspaceService) OpenWorkspace(workspaceIdentifier string) error {
	// Load the workspace file
	if err := lws.loadWorkspaceFile(workspaceIdentifier); err != nil {
		return fmt.Errorf("failed to load workspace file: %w", err)
	}

	lws.workspaceMu.Lock()
	lws.workspacePath = workspaceIdentifier
	lws.workspaceMu.Unlock()

	// Update window title with workspace filename
	lws.updateWindowTitle()

	lws.app.Log("info", fmt.Sprintf("Workspace loaded: %s", workspaceIdentifier))
	return nil
}

// CloseWorkspace closes the active workspace and clears all annotations
func (lws *LocalWorkspaceService) CloseWorkspace() error {
	if lws.ctx == nil {
		return errors.New("service not initialized")
	}

	lws.workspaceMu.Lock()
	lws.workspacePath = ""
	lws.annotationsMap = make(map[string]map[int]*interfaces.RowAnnotation)
	lws.fileData = make(map[string]*interfaces.WorkspaceFile)
	lws.annotationsByID = make(map[string]*annotationLocation)
	lws.workspaceMu.Unlock()

	// Update window title to default
	lws.updateWindowTitle()

	// Invalidate annotation caches to clear old workspace data
	lws.invalidateAnnotationCaches()

	lws.app.Log("info", "Workspace closed")
	return nil
}

// IsWorkspaceOpen returns true if a workspace file is currently open
func (lws *LocalWorkspaceService) IsWorkspaceOpen() bool {
	return lws.workspacePath != ""
}

// GetWorkspaceIdentifier returns the path of the currently open workspace file
func (lws *LocalWorkspaceService) GetWorkspaceIdentifier() string {
	return lws.workspacePath
}

// GetWorkspaceName returns the name of the currently open workspace
func (lws *LocalWorkspaceService) GetWorkspaceName() string {
	lws.workspaceMu.RLock()
	workspacePath := lws.workspacePath
	lws.workspaceMu.RUnlock()

	if workspacePath == "" {
		return ""
	}

	// Extract filename without extension from the workspace path
	filename := filepath.Base(workspacePath)
	if strings.HasSuffix(filename, ".breachline") {
		return strings.TrimSuffix(filename, ".breachline")
	}
	return filename
}

// IsRemoteWorkspace returns false for local workspaces
func (lws *LocalWorkspaceService) IsRemoteWorkspace() bool {
	return false
}

// updateWindowTitle updates the window title based on workspace state
func (lws *LocalWorkspaceService) updateWindowTitle() {
	if lws.ctx == nil {
		return
	}

	lws.workspaceMu.RLock()
	workspacePath := lws.workspacePath
	lws.workspaceMu.RUnlock()

	var title string
	if workspacePath == "" {
		title = "BreachLine"
	} else {
		// Extract just the filename from the path and remove extension
		filename := filepath.Base(workspacePath)
		ext := filepath.Ext(filename)
		if ext != "" {
			filename = strings.TrimSuffix(filename, ext)
		}
		title = fmt.Sprintf("BreachLine: %s", filename)
	}

	runtime.WindowSetTitle(lws.ctx, title)
}

// loadWorkspaceFile reads and parses a workspace file
func (lws *LocalWorkspaceService) loadWorkspaceFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var workspace WorkspaceConfig
	if err := yaml.Unmarshal(data, &workspace); err != nil {
		return err
	}

	lws.workspaceMu.Lock()
	defer lws.workspaceMu.Unlock()

	// Load the hash key from workspace file
	if workspace.HashKey != "" {
		decodedKey, err := base64.StdEncoding.DecodeString(workspace.HashKey)
		if err != nil {
			return fmt.Errorf("failed to decode hash key: %w", err)
		}
		if len(decodedKey) != 32 {
			return fmt.Errorf("invalid hash key length: expected 32 bytes, got %d", len(decodedKey))
		}
		lws.hashKey = decodedKey
	}

	// Build the annotations map for fast lookup
	// Index by composite key (fileHash+jpath) and row index for efficient O(1) matching
	lws.annotationsMap = make(map[string]map[int]*interfaces.RowAnnotation)
	lws.fileData = make(map[string]*interfaces.WorkspaceFile)
	lws.annotationsByID = make(map[string]*annotationLocation)

	for _, file := range workspace.Files {
		if file.FileHash == "" {
			continue
		}
		// Create composite key from file hash and options
		compositeKey := makeCompositeKey(file.FileHash, file.Options)

		if lws.annotationsMap[compositeKey] == nil {
			lws.annotationsMap[compositeKey] = make(map[int]*interfaces.RowAnnotation)
		}

		// Store file data using composite key
		lws.fileData[compositeKey] = &interfaces.WorkspaceFile{
			FilePath:    file.FilePath,
			FileHash:    file.FileHash,
			Options:     file.Options,
			Description: file.Description,
			Annotations: file.Annotations,
		}

		// Only process annotations if they exist for this file
		for i, annot := range file.Annotations {
			// Generate annotation ID if missing (migration for old workspace files)
			if annot.AnnotationID == "" {
				annot.AnnotationID = GenerateAnnotationID()
				file.Annotations[i] = annot // Update the annotation with generated ID
			}

			// Store annotation by row index for O(1) lookup
			annotCopy := annot // Create a copy to avoid issues with loop variable
			lws.annotationsMap[compositeKey][annot.RowIndex] = &annotCopy

			// Add to ID index for O(1) deletion by ID
			lws.annotationsByID[annot.AnnotationID] = &annotationLocation{
				compositeKey: compositeKey,
				rowIndex:     annot.RowIndex,
			}
		}
	}

	return nil
}

// NOTE: Deprecated column hash helper functions (getFirstColumnHash, extractHashFromColumnHash)
// have been removed. Annotations are now identified by RowIndex instead.

// GetWorkspaceFiles returns a list of all files tracked in the workspace
func (lws *LocalWorkspaceService) GetWorkspaceFiles() ([]*interfaces.WorkspaceFile, error) {
	lws.workspaceMu.RLock()
	workspacePath := lws.workspacePath
	fileData := lws.fileData
	annotationsMap := lws.annotationsMap
	lws.workspaceMu.RUnlock()

	if workspacePath == "" {
		return nil, errors.New("no workspace is open")
	}

	// Get an array of all files in workspace from FileData
	var files []*interfaces.WorkspaceFile
	for compositeKey, file := range fileData {
		// Count annotations for this file (now a simple map length)
		annotationCount := 0
		if fileAnnotations, exists := annotationsMap[compositeKey]; exists {
			annotationCount = len(fileAnnotations)
		}

		// Create a copy of the file with the annotation count populated
		fileCopy := *file
		fileCopy.AnnotationCount = annotationCount
		files = append(files, &fileCopy)
	}

	// Sort files: located files first (those with RelativePath), then unlocated files
	// Within each group, sort by FilePath, then FileHash, then Options.Key() for consistent ordering
	sort.Slice(files, func(i, j int) bool {
		// Check if files are located (have a RelativePath)
		iLocated := files[i].RelativePath != ""
		jLocated := files[j].RelativePath != ""

		// Located files come before unlocated files
		if iLocated != jLocated {
			return iLocated
		}

		// Within same location status, sort by other fields
		if files[i].FilePath != files[j].FilePath {
			return files[i].FilePath < files[j].FilePath
		}
		if files[i].FileHash != files[j].FileHash {
			return files[i].FileHash < files[j].FileHash
		}
		// Sort by Options.Key() for consistent ordering
		return files[i].Options.Key() < files[j].Options.Key()
	})

	return files, nil
}

func (lws *LocalWorkspaceService) GetWorkspaceFile(fileHash string, opts interfaces.FileOptions) (*interfaces.WorkspaceFile, error) {
	file := lws.fileData[makeCompositeKey(fileHash, opts)]
	if file == nil {
		return nil, errors.New("file not in workspace")
	}
	return file, nil
}

// AddFileToWorkspace adds a file to the workspace with its hash and options
func (lws *LocalWorkspaceService) AddFileToWorkspace(fileHash string, opts interfaces.FileOptions, filePath string, description string) error {
	if !lws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	lws.workspaceMu.Lock()
	defer lws.workspaceMu.Unlock()

	if lws.workspacePath == "" {
		return errors.New("no workspace is open")
	}

	// Generate hash key if not already present
	if lws.hashKey == nil {
		hashKey, err := GenerateHashKey()
		if err != nil {
			return fmt.Errorf("failed to generate hash key: %w", err)
		}
		lws.hashKey = hashKey
	}

	// Create composite key from file hash and options
	compositeKey := makeCompositeKey(fileHash, opts)

	// Store file data using composite key
	lws.fileData[compositeKey] = &interfaces.WorkspaceFile{
		FileHash:    fileHash,
		Options:     opts,
		FilePath:    filePath,
		Description: description,
	}

	// Initialize annotations map for this file+options combination if it doesn't exist
	if lws.annotationsMap[compositeKey] == nil {
		lws.annotationsMap[compositeKey] = make(map[int]*interfaces.RowAnnotation)
	}

	// Save the workspace file with updated file entry
	if err := lws.saveWorkspaceFileUnlocked(); err != nil {
		return fmt.Errorf("failed to save workspace: %w", err)
	}

	// Emit workspace updated event to notify frontend
	if lws.ctx != nil {
		runtime.EventsEmit(lws.ctx, "workspace:updated")
	}

	msg := fmt.Sprintf("Added file to workspace: %s", fileHash)
	if opts.JPath != "" {
		msg += fmt.Sprintf(" (JSONPath: %s)", opts.JPath)
	}
	lws.app.Log("info", msg)
	return nil
}

// UpdateFileDescription updates the description for a file in the workspace
func (lws *LocalWorkspaceService) UpdateFileDescription(fileHash string, opts interfaces.FileOptions, description string) error {
	if !lws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	lws.workspaceMu.Lock()
	defer lws.workspaceMu.Unlock()

	if lws.workspacePath == "" {
		return errors.New("no workspace is open")
	}

	// Create composite key from file hash and options
	compositeKey := makeCompositeKey(fileHash, opts)

	// Check if file exists in workspace
	if _, ok := lws.annotationsMap[compositeKey]; !ok {
		return errors.New("file not found in workspace")
	}

	// Update file data with description
	if lws.fileData[compositeKey] == nil {
		return errors.New("file not found in workspace")
	}
	lws.fileData[compositeKey].Description = description

	// Save the workspace file with updated description
	if err := lws.saveWorkspaceFileUnlocked(); err != nil {
		return fmt.Errorf("failed to save workspace: %w", err)
	}

	// Emit workspace updated event to notify frontend
	if lws.ctx != nil {
		runtime.EventsEmit(lws.ctx, "workspace:updated")
	}

	lws.app.Log("info", fmt.Sprintf("Updated description for file: %s", fileHash))
	return nil
}

// RemoveFileFromWorkspace removes a file from the workspace
func (lws *LocalWorkspaceService) RemoveFileFromWorkspace(fileHash string, opts interfaces.FileOptions) error {
	if !lws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	lws.workspaceMu.Lock()
	defer lws.workspaceMu.Unlock()

	if lws.workspacePath == "" {
		return errors.New("no workspace is open")
	}

	// Create composite key from file hash and options
	compositeKey := makeCompositeKey(fileHash, opts)

	// Check if file exists in workspace
	if _, ok := lws.annotationsMap[compositeKey]; !ok {
		return errors.New("file not found in workspace")
	}

	// Remove file from annotations map
	delete(lws.annotationsMap, compositeKey)

	// Remove file data
	delete(lws.fileData, compositeKey)

	// Save the workspace file
	if err := lws.saveWorkspaceFileUnlocked(); err != nil {
		return fmt.Errorf("failed to save workspace: %w", err)
	}

	// Emit workspace updated event to notify frontend
	if lws.ctx != nil {
		runtime.EventsEmit(lws.ctx, "workspace:updated")
	}

	msg := fmt.Sprintf("Removed file from workspace: %s", fileHash)
	if opts.JPath != "" {
		msg += fmt.Sprintf(" (JSONPath: %s)", opts.JPath)
	}
	lws.app.Log("info", msg)
	return nil
}

// saveWorkspaceFileUnlocked is an internal helper that saves the workspace file
// Caller must hold at least a read lock on workspaceMu
func (lws *LocalWorkspaceService) saveWorkspaceFileUnlocked() error {
	if lws.workspacePath == "" {
		return errors.New("no workspace is open")
	}

	// Convert map back to workspace structure
	workspace := WorkspaceConfig{
		Files: make([]interfaces.WorkspaceFile, 0),
	}

	// Encode and store hash key if present
	if lws.hashKey != nil {
		workspace.HashKey = base64.StdEncoding.EncodeToString(lws.hashKey)
	}

	for compositeKey, file := range lws.fileData {
		var fileAnnotations []interfaces.RowAnnotation
		// Collect all annotations for this file (now stored by row index)
		for _, annot := range lws.annotationsMap[compositeKey] {
			if annot != nil {
				fileAnnotations = append(fileAnnotations, *annot)
			}
		}
		file.Annotations = fileAnnotations
		workspace.Files = append(workspace.Files, *file)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&workspace)
	if err != nil {
		return err
	}

	// Write to file
	if err := os.WriteFile(lws.workspacePath, data, 0644); err != nil {
		return err
	}

	return nil
}

// saveWorkspaceFile writes the current annotations to the workspace file
func (lws *LocalWorkspaceService) saveWorkspaceFile() error {
	lws.workspaceMu.RLock()
	defer lws.workspaceMu.RUnlock()
	return lws.saveWorkspaceFileUnlocked()
}

// AddAnnotations adds the same annotation to multiple rows
func (lws *LocalWorkspaceService) AddAnnotations(fileHash string, opts interfaces.FileOptions, rowIndices []int, timeField string, note string, color string, query string) error {
	if !lws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	hasWorkspace := lws.workspacePath != ""
	if !hasWorkspace {
		return errors.New("no workspace is open")
	}

	// Check if file is in workspace, add it if not
	files, err := lws.GetWorkspaceFiles()
	if err != nil {
		return fmt.Errorf("failed to get workspace files: %w", err)
	}

	// Check if the specific fileHash+opts combination exists in the workspace
	var workspaceFile *interfaces.WorkspaceFile
	for _, file := range files {
		if file.FileHash == fileHash && file.Options.Equals(opts) {
			workspaceFile = file
			break
		}
	}

	tab := lws.app.GetActiveTab()
	if workspaceFile == nil {
		// This file isn't in the workspace, add it so that we can annotate it
		// Get file path from active tab
		if tab == nil {
			return errors.New("file not open in active tab")
		}
		// Add file to workspace before annotating
		if err := lws.AddFileToWorkspace(fileHash, opts, tab.FilePath, ""); err != nil {
			return fmt.Errorf("failed to add file to workspace: %w", err)
		}
	}

	// Execute query with metadata - returns both original header and filtered rows in single read
	// This consolidates what was previously two separate file reads (header + data)
	result, err := lws.app.ExecuteQueryForTabWithMetadata(tab, query, timeField)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	originalHeader := result.OriginalHeader
	if len(originalHeader) == 0 {
		return fmt.Errorf("missing original header in query result")
	}
	queryRows := result.Rows

	// Default to grey if no color specified
	if color == "" {
		color = "grey"
	}

	// Process each row index using the StageResult which has Row objects with RowIndex
	lws.workspaceMu.Lock()
	compositeKey := makeCompositeKey(fileHash, opts)
	if lws.annotationsMap[compositeKey] == nil {
		lws.annotationsMap[compositeKey] = make(map[int]*interfaces.RowAnnotation)
	}

	// Get Row objects from StageResult which have the original RowIndex
	var stageRows []*interfaces.Row
	if result.StageResult != nil {
		stageRows = result.StageResult.Rows
	}

	successCount := 0
	for _, rowIndex := range rowIndices {
		if rowIndex < 0 || rowIndex >= len(queryRows) {
			continue // Skip invalid indices
		}

		// Get the original row index from the Row object
		var originalRowIndex int
		if stageRows != nil && rowIndex < len(stageRows) {
			originalRowIndex = stageRows[rowIndex].RowIndex
		} else {
			// Fallback: use the query result index if no StageResult available
			originalRowIndex = rowIndex
		}

		// Check if annotation already exists at this row index
		existing := lws.annotationsMap[compositeKey][originalRowIndex]

		// Determine annotation ID - reuse existing or generate new
		var annotationID string
		if existing != nil {
			annotationID = existing.AnnotationID
		}
		if annotationID == "" {
			annotationID = GenerateAnnotationID()
		}

		// Create annotation with ID and row index
		annotation := &interfaces.RowAnnotation{
			AnnotationID: annotationID,
			RowIndex:     originalRowIndex,
			Note:         note,
			Color:        color,
		}

		// Store annotation by row index (O(1) lookup)
		lws.annotationsMap[compositeKey][originalRowIndex] = annotation

		// Update ID index
		lws.annotationsByID[annotationID] = &annotationLocation{
			compositeKey: compositeKey,
			rowIndex:     originalRowIndex,
		}
		successCount++
	}
	lws.workspaceMu.Unlock()

	if successCount == 0 {
		return errors.New("no valid rows could be annotated")
	}

	// Save to file
	if err := lws.saveWorkspaceFile(); err != nil {
		return fmt.Errorf("failed to save workspace: %w", err)
	}

	// After successful annotation addition and before emitting workspace event
	lws.invalidateAnnotationCaches()

	// Emit workspace updated event to notify frontend
	if lws.ctx != nil {
		runtime.EventsEmit(lws.ctx, "workspace:updated")
	}

	lws.app.Log("info", fmt.Sprintf("Annotation added to %d row(s)", successCount))
	return nil
}

// AddAnnotationsWithRows adds the same annotation to multiple rows using pre-fetched row data
// This is an optimized version that avoids redundant query execution
func (lws *LocalWorkspaceService) AddAnnotationsWithRows(fileHash string, opts interfaces.FileOptions, rowIndices []int, rows [][]string, header []string, timeField string, note string, color string, query string) error {
	if !lws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	hasWorkspace := lws.workspacePath != ""
	if !hasWorkspace {
		return errors.New("no workspace is open")
	}

	// Check if file is in workspace, add it if not
	files, err := lws.GetWorkspaceFiles()
	if err != nil {
		return fmt.Errorf("failed to get workspace files: %w", err)
	}

	// Check if the specific fileHash+opts combination exists in the workspace
	var workspaceFile *interfaces.WorkspaceFile
	for _, file := range files {
		if file.FileHash == fileHash && file.Options.Equals(opts) {
			workspaceFile = file
			break
		}
	}

	tab := lws.app.GetActiveTab()
	if workspaceFile == nil {
		// This file isn't in the workspace, add it so that we can annotate it
		// Get file path from active tab
		if tab == nil {
			return errors.New("file not open in active tab")
		}
		// Add file to workspace before annotating
		if err := lws.AddFileToWorkspace(fileHash, opts, tab.FilePath, ""); err != nil {
			return fmt.Errorf("failed to add file to workspace: %w", err)
		}
	}

	// Use provided rows (no need to re-query)
	// NOTE: header is no longer needed for row-index-based annotations
	_ = header // Suppress unused variable warning (kept for interface compatibility)
	queryRows := rows

	// Default to grey if no color specified
	if color == "" {
		color = "grey"
	}

	// Process each row index
	// NOTE: rowIndices are expected to be original file row indices, not filtered query indices
	lws.workspaceMu.Lock()
	compositeKey := makeCompositeKey(fileHash, opts)
	if lws.annotationsMap[compositeKey] == nil {
		lws.annotationsMap[compositeKey] = make(map[int]*interfaces.RowAnnotation)
	}

	successCount := 0
	// rowIndices[i] contains the original file row index for rows[i]
	// These arrays are parallel - validate using array index, not row index value
	for i, rowIndex := range rowIndices {
		if rowIndex < 0 || i >= len(queryRows) {
			continue // Skip invalid entries
		}

		// rowIndex is the original file row index (used for storage)
		originalRowIndex := rowIndex

		// Check if annotation already exists at this row index
		existing := lws.annotationsMap[compositeKey][originalRowIndex]

		// Determine annotation ID - reuse existing or generate new
		var annotationID string
		if existing != nil {
			annotationID = existing.AnnotationID
		}
		if annotationID == "" {
			annotationID = GenerateAnnotationID()
		}

		// Create annotation with ID and row index
		annotation := &interfaces.RowAnnotation{
			AnnotationID: annotationID,
			RowIndex:     originalRowIndex,
			Note:         note,
			Color:        color,
		}

		// Store annotation by row index (O(1) lookup)
		lws.annotationsMap[compositeKey][originalRowIndex] = annotation

		// Update ID index
		lws.annotationsByID[annotationID] = &annotationLocation{
			compositeKey: compositeKey,
			rowIndex:     originalRowIndex,
		}
		successCount++
	}
	lws.workspaceMu.Unlock()

	if successCount == 0 {
		return errors.New("no valid rows could be annotated")
	}

	// Save to file
	if err := lws.saveWorkspaceFile(); err != nil {
		return fmt.Errorf("failed to save workspace: %w", err)
	}

	// After successful annotation addition and before emitting workspace event
	lws.invalidateAnnotationCaches()

	// Emit workspace updated event to notify frontend
	if lws.ctx != nil {
		runtime.EventsEmit(lws.ctx, "workspace:updated")
	}

	lws.app.Log("info", fmt.Sprintf("Annotation added to %d row(s)", successCount))
	return nil
}

// IsRowAnnotatedByIndex checks if a row at a specific index has an annotation
// This is the preferred method for row-index-based annotation lookup
func (lws *LocalWorkspaceService) IsRowAnnotatedByIndex(fileHash string, opts interfaces.FileOptions, rowIndex int) (bool, *interfaces.AnnotationResult) {
	lws.workspaceMu.RLock()
	defer lws.workspaceMu.RUnlock()

	// Check if we have annotations for this file+opts combination
	compositeKey := makeCompositeKey(fileHash, opts)
	fileAnnots, ok := lws.annotationsMap[compositeKey]
	if !ok {
		return false, nil
	}

	// O(1) lookup by row index
	annot, exists := fileAnnots[rowIndex]
	if !exists || annot == nil {
		return false, nil
	}

	return true, &interfaces.AnnotationResult{
		ID:    annot.AnnotationID,
		Note:  annot.Note,
		Color: annot.Color,
	}
}

// HasAnnotationsForFile returns true if any annotations exist for the file
// This is used for early-exit optimization in the annotated query
func (lws *LocalWorkspaceService) HasAnnotationsForFile(fileHash string, opts interfaces.FileOptions) bool {
	lws.workspaceMu.RLock()
	defer lws.workspaceMu.RUnlock()

	compositeKey := makeCompositeKey(fileHash, opts)
	fileAnnots, ok := lws.annotationsMap[compositeKey]
	if !ok {
		return false
	}

	// Simply check if there are any annotations (map length > 0)
	return len(fileAnnots) > 0
}

// IsRowAnnotatedBatch checks multiple rows for annotations using row indices
// This is a highly optimized O(1) lookup per row using the RowIndex field
func (lws *LocalWorkspaceService) IsRowAnnotatedBatch(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) []bool {
	results := make([]bool, len(rows))
	_ = hashKey // hashKey is no longer needed for row-index-based lookups

	if len(rows) == 0 {
		return results
	}

	lws.workspaceMu.RLock()
	compositeKey := makeCompositeKey(fileHash, opts)
	fileAnnots, ok := lws.annotationsMap[compositeKey]
	lws.workspaceMu.RUnlock()

	if !ok || len(fileAnnots) == 0 {
		return results // All false
	}

	// Simple O(1) lookup per row using RowIndex - no parallelization needed
	// because map lookups are already very fast
	for i, row := range rows {
		if row == nil {
			continue
		}
		// O(1) lookup by row index
		_, exists := fileAnnots[row.RowIndex]
		results[i] = exists
	}

	return results
}

// IsRowAnnotatedBatchWithColors checks multiple rows for annotations using row indices
// and returns both the boolean status and colors in one pass
func (lws *LocalWorkspaceService) IsRowAnnotatedBatchWithColors(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) ([]bool, []string) {
	results := make([]bool, len(rows))
	colors := make([]string, len(rows))
	_ = hashKey // hashKey is no longer needed for row-index-based lookups

	if len(rows) == 0 {
		return results, colors
	}

	lws.workspaceMu.RLock()
	compositeKey := makeCompositeKey(fileHash, opts)
	fileAnnots, ok := lws.annotationsMap[compositeKey]
	lws.workspaceMu.RUnlock()

	if !ok || len(fileAnnots) == 0 {
		return results, colors // All false, empty colors
	}

	// Simple O(1) lookup per row using RowIndex
	for i, row := range rows {
		if row == nil {
			continue
		}
		// O(1) lookup by row index
		if annot, exists := fileAnnots[row.RowIndex]; exists && annot != nil {
			results[i] = true
			colors[i] = annot.Color
		}
	}

	return results, colors
}

// GetRowAnnotations retrieves the annotations for multiple rows
func (lws *LocalWorkspaceService) GetRowAnnotations(fileHash string, opts interfaces.FileOptions, rowIndices []int, query string, timeField string) (map[int]*interfaces.AnnotationResult, error) {
	workspaceFile, err := lws.GetWorkspaceFile(fileHash, opts)
	if workspaceFile == nil || err != nil {
		return nil, errors.New("file not in workspace")
	}

	tab := lws.app.GetActiveTab()
	if tab == nil || tab.FilePath != workspaceFile.FilePath {
		return nil, errors.New("file not open in active tab")
	}

	// Execute query - returns FULL rows with metadata
	result, err := lws.app.ExecuteQueryForTabWithMetadata(tab, query, timeField)
	if err != nil {
		return nil, err
	}

	// Get Row objects from StageResult which have the original RowIndex
	var stageRows []*interfaces.Row
	if result.StageResult != nil {
		stageRows = result.StageResult.Rows
	}

	results := make(map[int]*interfaces.AnnotationResult)
	for _, rowIndex := range rowIndices {
		if rowIndex < 0 || rowIndex >= len(result.Rows) {
			results[rowIndex] = &interfaces.AnnotationResult{Note: "", Color: ""}
			continue
		}

		// Get original row index from the Row object
		var originalRowIndex int
		if stageRows != nil && rowIndex < len(stageRows) {
			originalRowIndex = stageRows[rowIndex].RowIndex
		} else {
			originalRowIndex = rowIndex
		}

		// Use row index for O(1) annotation lookup
		annotated, annotResult := lws.IsRowAnnotatedByIndex(fileHash, opts, originalRowIndex)
		if annotated {
			results[rowIndex] = annotResult
		} else {
			results[rowIndex] = &interfaces.AnnotationResult{Note: "", Color: ""}
		}
	}

	return results, nil
}

// GetRowAnnotationsWithRows retrieves annotations using pre-fetched row data
// This is an optimized version that avoids redundant query execution
func (lws *LocalWorkspaceService) GetRowAnnotationsWithRows(fileHash string, opts interfaces.FileOptions, rowIndices []int, rows [][]string, header []string) (map[int]*interfaces.AnnotationResult, error) {
	workspaceFile, err := lws.GetWorkspaceFile(fileHash, opts)
	if workspaceFile == nil || err != nil {
		return nil, errors.New("file not in workspace")
	}

	results := make(map[int]*interfaces.AnnotationResult)

	// Early exit if no annotations for this file
	if !lws.HasAnnotationsForFile(fileHash, opts) {
		for _, rowIndex := range rowIndices {
			results[rowIndex] = &interfaces.AnnotationResult{Note: "", Color: ""}
		}
		return results, nil
	}

	// Convert to Row format for batch check
	// IMPORTANT: Set RowIndex from rowIndices for row-index-based annotation matching
	interfaceRows := make([]*interfaces.Row, len(rows))
	for i, row := range rows {
		rowIdx := 0
		if i < len(rowIndices) {
			rowIdx = rowIndices[i]
		}
		interfaceRows[i] = &interfaces.Row{DisplayIndex: -1, Data: row, RowIndex: rowIdx}
	}

	// Get hash key for batch check
	hashKey := lws.GetHashKey()
	if hashKey == nil {
		return nil, errors.New("workspace hash key not available")
	}

	// Single batch check (parallel processing) to determine which rows are annotated
	annotatedFlags := lws.IsRowAnnotatedBatch(fileHash, opts, interfaceRows, hashKey)

	// Only fetch annotation details for rows that are annotated
	for i, rowIndex := range rowIndices {
		if i >= len(rows) {
			results[rowIndex] = &interfaces.AnnotationResult{Note: "", Color: ""}
			continue
		}

		if annotatedFlags[i] {
			// Use IsRowAnnotatedByIndex for proper row-index-based lookup
			// rowIndex contains the actual file row index (passed by caller)
			_, annotResult := lws.IsRowAnnotatedByIndex(fileHash, opts, rowIndex)
			if annotResult != nil {
				results[rowIndex] = annotResult
			} else {
				results[rowIndex] = &interfaces.AnnotationResult{Note: "", Color: ""}
			}
		} else {
			results[rowIndex] = &interfaces.AnnotationResult{Note: "", Color: ""}
		}
	}

	return results, nil
}

// GetRowAnnotation retrieves the annotation for a specific row
func (lws *LocalWorkspaceService) GetRowAnnotation(fileHash string, opts interfaces.FileOptions, rowIndex int, query string, timeField string) (*interfaces.AnnotationResult, error) {
	workspaceFile, err := lws.GetWorkspaceFile(fileHash, opts)
	if workspaceFile == nil || err != nil {
		return nil, errors.New("file not in workspace")
	}

	tab := lws.app.GetActiveTab()
	if tab == nil || tab.FilePath != workspaceFile.FilePath {
		return nil, errors.New("file not open in active tab")
	}

	// Execute query - returns FULL rows with metadata
	result, err := lws.app.ExecuteQueryForTabWithMetadata(tab, query, timeField)
	if err != nil {
		return nil, err
	}

	if rowIndex < 0 || rowIndex >= len(result.Rows) {
		return nil, errors.New("invalid row index")
	}

	// Get actual row index from StageResult
	var actualRowIndex int
	if result.StageResult != nil && rowIndex < len(result.StageResult.Rows) {
		actualRowIndex = result.StageResult.Rows[rowIndex].RowIndex
	} else {
		actualRowIndex = rowIndex
	}

	// Use row-index-based lookup
	annotated, annotResult := lws.IsRowAnnotatedByIndex(workspaceFile.FileHash, opts, actualRowIndex)
	if annotated {
		return annotResult, nil
	}
	return &interfaces.AnnotationResult{Note: "", Color: ""}, nil
}

// DeleteRowAnnotations removes annotations for multiple rows
// This method uses StageResult.Rows which have the correct RowIndex from the original file
func (lws *LocalWorkspaceService) DeleteRowAnnotations(fileHash string, opts interfaces.FileOptions, rowIndices []int, timeField string, query string) error {
	if !lws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	lws.workspaceMu.RLock()
	hasWorkspace := lws.workspacePath != ""
	hashKey := lws.hashKey
	lws.workspaceMu.RUnlock()

	if !hasWorkspace {
		return errors.New("no workspace is open")
	}

	workspaceFile, err := lws.GetWorkspaceFile(fileHash, opts)
	if workspaceFile == nil || err != nil {
		return errors.New("file not in workspace")
	}

	tab := lws.app.GetActiveTab()
	if tab == nil || tab.FilePath != workspaceFile.FilePath {
		return errors.New("file not open in active tab")
	}

	// Execute query with metadata - returns StageResult with Rows that have correct RowIndex
	result, err := lws.app.ExecuteQueryForTabWithMetadata(tab, query, timeField)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// CRITICAL: Use StageResult.Rows which have the correct RowIndex from the original file
	// The rowIndices parameter contains DISPLAY indices (0, 1, 2...) which may differ from
	// the actual file row indices when data is sorted/filtered
	if result.StageResult == nil || len(result.StageResult.Rows) == 0 {
		return fmt.Errorf("no stage result available for annotation deletion")
	}

	stageRows := result.StageResult.Rows

	lws.app.Log("debug", fmt.Sprintf("Processing deletion for %d display indices, total rows in result: %d", len(rowIndices), len(stageRows)))

	// Extract the actual Row objects from StageResult using display indices
	// These Row objects have the correct RowIndex from the original file
	interfaceRows := make([]*interfaces.Row, 0, len(rowIndices))
	for _, displayIndex := range rowIndices {
		if displayIndex >= 0 && displayIndex < len(stageRows) && stageRows[displayIndex] != nil {
			interfaceRows = append(interfaceRows, stageRows[displayIndex])
			lws.app.Log("debug", fmt.Sprintf("Display index %d maps to file row index %d", displayIndex, stageRows[displayIndex].RowIndex))
		}
	}

	if len(interfaceRows) == 0 {
		return errors.New("no valid rows to delete annotations from")
	}

	// Batch get annotation info (includes IDs) using correct RowIndex
	annotationInfos := lws.IsRowAnnotatedBatchWithInfo(fileHash, opts, interfaceRows, hashKey)

	// Collect unique annotation IDs to delete
	annotationIDsToDelete := make(map[string]bool)
	for _, info := range annotationInfos {
		if info != nil && info.ID != "" {
			annotationIDsToDelete[info.ID] = true
		}
	}

	if len(annotationIDsToDelete) == 0 {
		return errors.New("no annotations found to delete")
	}

	lws.app.Log("debug", fmt.Sprintf("Found %d unique annotations to delete from %d rows", len(annotationIDsToDelete), len(interfaceRows)))

	// OPTIMIZED: Direct deletion by annotation ID - O(1) per annotation
	lws.workspaceMu.Lock()
	deleteCount := 0
	compositeKey := makeCompositeKey(fileHash, opts)

	fileAnnots, ok := lws.annotationsMap[compositeKey]
	if !ok {
		lws.workspaceMu.Unlock()
		return errors.New("no annotations found for file")
	}

	// Delete each annotation by ID using O(1) lookup
	for annotationID := range annotationIDsToDelete {
		location, ok := lws.annotationsByID[annotationID]
		if !ok || location.compositeKey != compositeKey {
			continue
		}

		// Delete from annotations map using row index
		delete(fileAnnots, location.rowIndex)
		// Delete from ID index
		delete(lws.annotationsByID, annotationID)
		deleteCount++
	}
	lws.workspaceMu.Unlock()

	if deleteCount == 0 {
		return errors.New("no valid annotations could be deleted")
	}

	// Save to file
	if err := lws.saveWorkspaceFile(); err != nil {
		return fmt.Errorf("failed to save workspace: %w", err)
	}

	// Invalidate annotation caches
	lws.invalidateAnnotationCaches()

	// Emit workspace updated event to notify frontend
	if lws.ctx != nil {
		runtime.EventsEmit(lws.ctx, "workspace:updated")
	}

	lws.app.Log("info", fmt.Sprintf("Annotation deleted for %d row(s)", deleteCount))
	return nil
}

// DeleteRowAnnotationsWithRows removes annotations for multiple rows using pre-fetched row data
// This is an optimized version that uses annotation IDs for O(1) deletion
// NOTE: rowIndices should contain actual file row indices (not display indices)
// The rows array should be positionally aligned with rowIndices (rows[i] corresponds to rowIndices[i])
func (lws *LocalWorkspaceService) DeleteRowAnnotationsWithRows(fileHash string, opts interfaces.FileOptions, rowIndices []int, rows [][]string, header []string, timeField string, query string) error {
	if !lws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	lws.workspaceMu.RLock()
	hasWorkspace := lws.workspacePath != ""
	hashKey := lws.hashKey
	lws.workspaceMu.RUnlock()

	if !hasWorkspace {
		return errors.New("no workspace is open")
	}

	workspaceFile, err := lws.GetWorkspaceFile(fileHash, opts)
	if workspaceFile == nil || err != nil {
		return errors.New("file not in workspace")
	}

	lws.app.Log("debug", fmt.Sprintf("Processing deletion for %d row indices with pre-fetched rows", len(rowIndices)))

	// Convert rows to interfaces.Row for batch processing
	// rowIndices contains ACTUAL FILE ROW INDICES (not display indices)
	// rows[i] contains the data for rowIndices[i]
	interfaceRows := make([]*interfaces.Row, 0, len(rowIndices))
	for i, actualRowIndex := range rowIndices {
		if i >= 0 && i < len(rows) && len(rows[i]) > 0 {
			// Use actualRowIndex as the RowIndex for annotation lookup
			interfaceRows = append(interfaceRows, &interfaces.Row{DisplayIndex: -1, Data: rows[i], RowIndex: actualRowIndex})
			lws.app.Log("debug", fmt.Sprintf("Row %d has actual file row index %d", i, actualRowIndex))
		}
	}

	if len(interfaceRows) == 0 {
		return errors.New("no valid rows to delete annotations from")
	}

	// Batch get annotation info (includes IDs) - this is parallelized
	annotationInfos := lws.IsRowAnnotatedBatchWithInfo(fileHash, opts, interfaceRows, hashKey)

	// Collect unique annotation IDs to delete
	annotationIDsToDelete := make(map[string]bool)
	for _, info := range annotationInfos {
		if info != nil && info.ID != "" {
			annotationIDsToDelete[info.ID] = true
		}
	}

	if len(annotationIDsToDelete) == 0 {
		return errors.New("no annotations found to delete")
	}

	lws.app.Log("debug", fmt.Sprintf("Found %d unique annotations to delete from %d rows", len(annotationIDsToDelete), len(interfaceRows)))

	// OPTIMIZED: Direct deletion by annotation ID - O(1) per annotation
	lws.workspaceMu.Lock()
	deleteCount := 0
	compositeKey := makeCompositeKey(fileHash, opts)

	fileAnnots, ok := lws.annotationsMap[compositeKey]
	if !ok {
		lws.workspaceMu.Unlock()
		return errors.New("no annotations found for file")
	}

	// Delete each annotation by ID using O(1) lookup
	for annotationID := range annotationIDsToDelete {
		location, ok := lws.annotationsByID[annotationID]
		if !ok || location.compositeKey != compositeKey {
			continue
		}

		// Delete from annotations map using row index
		delete(fileAnnots, location.rowIndex)
		// Delete from ID index
		delete(lws.annotationsByID, annotationID)
		deleteCount++
	}
	lws.workspaceMu.Unlock()

	if deleteCount == 0 {
		return errors.New("no valid annotations could be deleted")
	}

	// Save to file
	if err := lws.saveWorkspaceFile(); err != nil {
		return fmt.Errorf("failed to save workspace: %w", err)
	}

	// Invalidate annotation caches
	lws.invalidateAnnotationCaches()

	// Emit workspace updated event to notify frontend
	if lws.ctx != nil {
		runtime.EventsEmit(lws.ctx, "workspace:updated")
	}

	lws.app.Log("info", fmt.Sprintf("Annotation deleted for %d row(s) using pre-fetched data", deleteCount))
	return nil
}

// GetHashKey returns the workspace's hash key for file hashing
func (lws *LocalWorkspaceService) GetHashKey() []byte {
	lws.workspaceMu.RLock()
	defer lws.workspaceMu.RUnlock()

	if lws.hashKey == nil {
		// Generate hash key if not already present
		hashKey, err := GenerateHashKey()
		if err != nil {
			return nil
		}
		lws.hashKey = hashKey
	}

	return lws.hashKey
}

// ExportWorkspaceTimeline exports all annotated rows from all workspace files to a single CSV
func (lws *LocalWorkspaceService) ExportWorkspaceTimeline() error {
	if !lws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	lws.workspaceMu.RLock()
	hasWorkspace := lws.workspacePath != ""
	lws.workspaceMu.RUnlock()

	if !hasWorkspace {
		return errors.New("no workspace is open")
	}

	// Open save file dialog
	outputPath, err := runtime.SaveFileDialog(lws.ctx, runtime.SaveDialogOptions{
		Title:           "Export Workspace Timeline",
		DefaultFilename: "timeline_export.csv",
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV Files", Pattern: "*.csv"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to open save dialog: %w", err)
	}

	// User cancelled
	if outputPath == "" {
		return errors.New("export cancelled by user")
	}

	// Ensure .csv extension
	if !strings.HasSuffix(strings.ToLower(outputPath), ".csv") {
		outputPath = outputPath + ".csv"
	}

	lws.app.Log("info", "Exporting workspace timeline...")

	// Get effective settings for timestamp formatting
	effective := settings.GetEffectiveSettings()
	tzName := strings.TrimSpace(effective.DisplayTimezone)
	var displayLoc *time.Location
	switch strings.ToUpper(tzName) {
	case "", "LOCAL":
		displayLoc = time.Local
	case "UTC":
		displayLoc = time.UTC
	default:
		if l, err := time.LoadLocation(tzName); err == nil {
			displayLoc = l
		} else {
			displayLoc = time.Local
		}
	}

	// Create timestamp formatter function
	formatTimestamp := func(s string) string {
		ms, ok := timestamps.ParseTimestampMillis(s, nil)
		if !ok {
			return s // Return original if can't parse
		}
		t := time.UnixMilli(ms).In(displayLoc)
		// Convert pattern to Go layout
		toGoLayout := func(p string) string {
			p = strings.TrimSpace(p)
			if p == "" {
				return "2006-01-02 15:04:05"
			}
			r := strings.NewReplacer(
				"yyyy", "2006",
				"yy", "06",
				"MM", "01",
				"dd", "02",
				"HH", "15",
				"mm", "04",
				"ss", "05",
				"SSS", "000",
				"zzz", "MST",
			)
			return r.Replace(p)
		}
		pattern := strings.TrimSpace(effective.TimestampDisplayFormat)
		if pattern == "" {
			pattern = "yyyy-MM-dd HH:mm:ss"
		}
		layout := toGoLayout(pattern)
		return t.Format(layout)
	}

	// Get all files with annotations
	files, err := lws.GetWorkspaceFiles()
	if err != nil {
		return fmt.Errorf("failed to get workspace files: %w", err)
	}

	// Filter to only files with annotations
	var filesWithAnnotations []interfaces.WorkspaceFile
	for _, file := range files {
		if len(file.Annotations) > 0 {
			filesWithAnnotations = append(filesWithAnnotations, *file)
		}
	}

	if len(filesWithAnnotations) == 0 {
		return errors.New("no annotated files found in workspace")
	}

	// Collect all annotated rows from all files
	type AnnotatedRow struct {
		Timestamp       string
		FilePath        string
		FileDescription string
		Color           string
		Note            string
		Data            map[string]string // column name -> value
	}

	var allAnnotatedRows []AnnotatedRow
	var allColumnNames map[string]bool = make(map[string]bool)

	// Process each file
	for _, fileInfo := range filesWithAnnotations {
		lws.app.Log("info", fmt.Sprintf("Processing file: %s", fileInfo.FilePath))

		// Read the file data
		var header []string
		var rows [][]string

		// Detect file type and read accordingly
		fileType := fileloader.DetectFileType(fileInfo.FilePath)
		if fileType == fileloader.FileTypeJSON {
			if fileInfo.Options.JPath == "" {
				lws.app.Log("error", fmt.Sprintf("JSON file %s requires a JSONPath expression", fileInfo.FilePath))
				continue
			}
			header, err = lws.app.ReadJSONHeader(fileInfo.FilePath, fileInfo.Options.JPath)
			if err != nil {
				lws.app.Log("error", fmt.Sprintf("Failed to read header from %s: %v", fileInfo.FilePath, err))
				continue
			}
			// Read all rows using JSON reader
			reader, _, err := lws.app.GetJSONReader(fileInfo.FilePath, fileInfo.Options.JPath)
			if err != nil {
				lws.app.Log("error", fmt.Sprintf("Failed to read file %s: %v", fileInfo.FilePath, err))
				continue
			}
			// Skip header row and read all data rows
			_, _ = reader.Read() // skip header
			for {
				row, err := reader.Read()
				if err != nil {
					break
				}
				rows = append(rows, row)
			}
		} else {
			// CSV or XLSX
			header, err = lws.app.ReadHeader(fileInfo.FilePath)
			if err != nil {
				lws.app.Log("error", fmt.Sprintf("Failed to read header from %s: %v", fileInfo.FilePath, err))
				continue
			}
			// Read all rows
			reader, file, err := lws.app.GetReader(fileInfo.FilePath)
			if err != nil {
				lws.app.Log("error", fmt.Sprintf("Failed to read file %s: %v", fileInfo.FilePath, err))
				continue
			}
			if file != nil {
				defer file.Close()
			}
			// Skip header row and read all data rows
			_, _ = reader.Read() // skip header
			for {
				row, err := reader.Read()
				if err != nil {
					break
				}
				rows = append(rows, row)
			}
		}

		// Find timestamp column
		timestampIdx := timestamps.DetectTimestampIndex(header)
		if timestampIdx == -1 {
			lws.app.Log("error", fmt.Sprintf("No timestamp column found in file %s", fileInfo.FilePath))
			continue
		}

		// Check each row to see if it's annotated
		// Use row index for annotation lookup (row index = position in file, 0-indexed)
		for rowIndex, row := range rows {
			if len(row) == 0 {
				continue
			}

			isAnnotated, annotation := lws.IsRowAnnotatedByIndex(fileInfo.FileHash, fileInfo.Options, rowIndex)
			if isAnnotated {
				// Create annotated row with formatted timestamp and metadata
				formattedTimestamp := formatTimestamp(row[timestampIdx])
				annotatedRow := AnnotatedRow{
					Timestamp:       formattedTimestamp,
					FilePath:        fileInfo.FilePath,
					FileDescription: fileInfo.Description,
					Color:           annotation.Color,
					Note:            annotation.Note,
					Data:            make(map[string]string),
				}

				// Add all columns
				for i, value := range row {
					if i < len(header) {
						colName := header[i]
						colValue := value
						// Override timestamp column name to _TT_timestamp and use formatted value
						if i == timestampIdx {
							colName = "_TT_timestamp"
							colValue = formattedTimestamp
						}
						annotatedRow.Data[colName] = colValue
						allColumnNames[colName] = true
					}
				}

				allAnnotatedRows = append(allAnnotatedRows, annotatedRow)
			}
		}
	}

	if len(allAnnotatedRows) == 0 {
		return errors.New("no annotated rows found")
	}

	// Sort all annotated rows by timestamp in ascending order
	sort.Slice(allAnnotatedRows, func(i, j int) bool {
		// Parse timestamps back to compare them properly
		tiI, okI := timestamps.ParseTimestampMillis(allAnnotatedRows[i].Data["_TT_timestamp"], nil)
		tiJ, okJ := timestamps.ParseTimestampMillis(allAnnotatedRows[j].Data["_TT_timestamp"], nil)
		if okI && okJ {
			return tiI < tiJ
		}
		// Fallback to string comparison if parsing fails
		return allAnnotatedRows[i].Data["_TT_timestamp"] < allAnnotatedRows[j].Data["_TT_timestamp"]
	})

	// Build sorted column list: _TT_timestamp first, then metadata columns, then other columns alphabetically
	var dataColumnList []string
	for colName := range allColumnNames {
		if colName != "_TT_timestamp" {
			dataColumnList = append(dataColumnList, colName)
		}
	}
	sort.Strings(dataColumnList)
	// Final column order: timestamp, filepath, description, note, then data columns
	columnList := []string{"_TT_timestamp", "_TT_filepath", "_TT_file_description", "_TT_note"}
	columnList = append(columnList, dataColumnList...)

	// Write to CSV file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Write header
	headerLine := strings.Join(columnList, ",") + "\n"
	if _, err := file.WriteString(headerLine); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write rows
	for _, annotatedRow := range allAnnotatedRows {
		var rowValues []string
		for _, colName := range columnList {
			var value string
			// Get value from appropriate source
			switch colName {
			case "_TT_timestamp":
				value = annotatedRow.Data["_TT_timestamp"]
			case "_TT_filepath":
				value = annotatedRow.FilePath
			case "_TT_file_description":
				value = annotatedRow.FileDescription
			case "_TT_note":
				value = annotatedRow.Note
			default:
				value = annotatedRow.Data[colName]
			}
			// Escape CSV values if needed (simple escaping)
			if strings.Contains(value, ",") || strings.Contains(value, "\"") || strings.Contains(value, "\n") {
				value = "\"" + strings.ReplaceAll(value, "\"", "\"\"") + "\""
			}
			rowValues = append(rowValues, value)
		}
		rowLine := strings.Join(rowValues, ",") + "\n"
		if _, err := file.WriteString(rowLine); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	lws.app.Log("info", fmt.Sprintf("Exported %d annotated rows to %s", len(allAnnotatedRows), filepath.Base(outputPath)))
	return nil
}

// RefreshFileLocations is a no-op for local workspaces since they don't use remote file locations
func (lws *LocalWorkspaceService) RefreshFileLocations() error {
	// Local workspaces don't have remote file locations to refresh
	return nil
}

// RegisterCacheInvalidator registers a cache invalidator for annotation-dependent caches
func (lws *LocalWorkspaceService) RegisterCacheInvalidator(invalidator CacheInvalidator) {
	lws.invalidatorMu.Lock()
	defer lws.invalidatorMu.Unlock()
	lws.cacheInvalidators = append(lws.cacheInvalidators, invalidator)
	lws.app.Log("debug", fmt.Sprintf("[CACHE_INVALIDATOR_REGISTERED] Registered cache invalidator (total: %d)", len(lws.cacheInvalidators)))
}

// invalidateAnnotationCaches triggers invalidation of all registered annotation-dependent caches
func (lws *LocalWorkspaceService) invalidateAnnotationCaches() {
	lws.invalidatorMu.RLock()
	invalidators := make([]CacheInvalidator, len(lws.cacheInvalidators))
	copy(invalidators, lws.cacheInvalidators)
	lws.invalidatorMu.RUnlock()

	lws.app.Log("debug", fmt.Sprintf("[INVALIDATE_ANNOTATION_START] Found %d registered cache invalidators", len(invalidators)))

	for i, invalidator := range invalidators {
		lws.app.Log("debug", fmt.Sprintf("[INVALIDATE_ANNOTATION_CALL] Calling invalidator %d/%d", i+1, len(invalidators)))
		invalidator.InvalidateAnnotationCaches("local") // Use "local" as workspace ID
		lws.app.Log("debug", fmt.Sprintf("[INVALIDATE_ANNOTATION_DONE] Completed invalidator %d/%d", i+1, len(invalidators)))
	}

	lws.app.Log("debug", "Invalidated annotation-dependent caches for local workspace")
}

// DeleteAnnotationByID deletes an annotation by its ID (O(1) lookup)
func (lws *LocalWorkspaceService) DeleteAnnotationByID(annotationID string) error {
	if !lws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	lws.workspaceMu.Lock()
	defer lws.workspaceMu.Unlock()

	if lws.workspacePath == "" {
		return errors.New("no workspace is open")
	}

	// Look up annotation location by ID
	location, ok := lws.annotationsByID[annotationID]
	if !ok {
		return fmt.Errorf("annotation not found: %s", annotationID)
	}

	// Get the annotations map for this file
	fileAnnots, ok := lws.annotationsMap[location.compositeKey]
	if !ok {
		return fmt.Errorf("file annotations not found for annotation: %s", annotationID)
	}

	// Verify the annotation exists at the expected row index
	annot, ok := fileAnnots[location.rowIndex]
	if !ok || annot == nil {
		return fmt.Errorf("annotation not found at row index: %s", annotationID)
	}

	// Verify the annotation ID matches (safety check)
	if annot.AnnotationID != annotationID {
		return fmt.Errorf("annotation ID mismatch at row index: %s", annotationID)
	}

	// Remove the annotation from the map (O(1) operation)
	delete(fileAnnots, location.rowIndex)

	// Remove from ID index
	delete(lws.annotationsByID, annotationID)

	// Save workspace file (unlocked version since we hold the lock)
	if err := lws.saveWorkspaceFileUnlocked(); err != nil {
		return fmt.Errorf("failed to save workspace: %w", err)
	}

	// Invalidate caches
	lws.invalidateAnnotationCaches()

	// Emit workspace updated event
	if lws.ctx != nil {
		runtime.EventsEmit(lws.ctx, "workspace:updated")
	}

	lws.app.Log("info", fmt.Sprintf("Deleted annotation: %s", annotationID))
	return nil
}

// GetAnnotationByID retrieves an annotation by its ID
func (lws *LocalWorkspaceService) GetAnnotationByID(annotationID string) (*interfaces.AnnotationResult, error) {
	lws.workspaceMu.RLock()
	defer lws.workspaceMu.RUnlock()

	if lws.workspacePath == "" {
		return nil, errors.New("no workspace is open")
	}

	// Look up annotation location by ID
	location, ok := lws.annotationsByID[annotationID]
	if !ok {
		return nil, fmt.Errorf("annotation not found: %s", annotationID)
	}

	// Get the annotation from the map using row index
	fileAnnots, ok := lws.annotationsMap[location.compositeKey]
	if !ok {
		return nil, fmt.Errorf("file annotations not found for annotation: %s", annotationID)
	}

	annot, ok := fileAnnots[location.rowIndex]
	if !ok || annot == nil {
		return nil, fmt.Errorf("annotation not found at row index: %s", annotationID)
	}

	// Verify the ID matches (safety check)
	if annot.AnnotationID != annotationID {
		return nil, fmt.Errorf("annotation ID mismatch at row index: %s", annotationID)
	}

	return &interfaces.AnnotationResult{
		ID:    annot.AnnotationID,
		Note:  annot.Note,
		Color: annot.Color,
	}, nil
}

// IsRowAnnotatedBatchWithInfo returns full annotation info for batch of rows
// This populates Row.Annotation for caching and allows ID-based operations
func (lws *LocalWorkspaceService) IsRowAnnotatedBatchWithInfo(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) []*interfaces.RowAnnotationInfo {
	results := make([]*interfaces.RowAnnotationInfo, len(rows))
	_ = hashKey // hashKey is no longer needed for row-index-based lookups

	if len(rows) == 0 {
		return results
	}

	lws.workspaceMu.RLock()
	compositeKey := makeCompositeKey(fileHash, opts)
	fileAnnots, ok := lws.annotationsMap[compositeKey]
	lws.workspaceMu.RUnlock()

	if !ok || len(fileAnnots) == 0 {
		return results // All nil
	}

	// Simple O(1) lookup per row using RowIndex - no parallelization needed
	for i, row := range rows {
		if row == nil {
			continue
		}

		// O(1) lookup by row index
		if annot, exists := fileAnnots[row.RowIndex]; exists && annot != nil {
			info := &interfaces.RowAnnotationInfo{
				ID:    annot.AnnotationID,
				Color: annot.Color,
				Note:  annot.Note,
			}
			results[i] = info
			// Also populate the Row's cached annotation
			row.Annotation = info
		}
	}

	return results
}

// GetFileAnnotations returns all annotations for a file
// Used by the annotation panel to display all annotations
func (lws *LocalWorkspaceService) GetFileAnnotations(fileHash string, opts interfaces.FileOptions) ([]*interfaces.FileAnnotationInfo, error) {
	lws.workspaceMu.RLock()
	defer lws.workspaceMu.RUnlock()

	compositeKey := makeCompositeKey(fileHash, opts)
	fileAnnots, ok := lws.annotationsMap[compositeKey]
	if !ok || len(fileAnnots) == 0 {
		return []*interfaces.FileAnnotationInfo{}, nil
	}

	results := make([]*interfaces.FileAnnotationInfo, 0, len(fileAnnots))
	for rowIndex, annot := range fileAnnots {
		if annot == nil {
			continue
		}
		results = append(results, &interfaces.FileAnnotationInfo{
			OriginalRowIndex: rowIndex,
			DisplayRowIndex:  -1, // Will be mapped by caller using tab's index mapping
			Note:             annot.Note,
			Color:            annot.Color,
		})
	}

	return results, nil
}
