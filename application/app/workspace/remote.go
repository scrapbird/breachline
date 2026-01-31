package workspace

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

	"github.com/scrapbird/breachline/infra/sync-api/src/api"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// RemoteWorkspace represents a workspace from the sync API
type RemoteWorkspace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	FileCount   int    `json:"file_count"`
}

// RemoteWorkspaceDetails represents detailed workspace information
type RemoteWorkspaceDetails struct {
	WorkspaceID string                 `json:"workspace_id"`
	Name        string                 `json:"name"`
	OwnerID     string                 `json:"owner_id"`
	IsShared    bool                   `json:"is_shared"`
	MemberCount int                    `json:"member_count"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
	Version     int                    `json:"version"`
	Statistics  map[string]interface{} `json:"statistics,omitempty"`
}

// RemoteAnnotationOptions contains file options for annotations from the sync API
type RemoteAnnotationOptions struct {
	JPath                  string `json:"jpath,omitempty"`
	NoHeaderRow            bool   `json:"noHeaderRow,omitempty"`
	IngestTimezoneOverride string `json:"ingestTimezoneOverride,omitempty"`
	// Directory loading options
	IsDirectory         bool   `json:"isDirectory,omitempty"`
	FilePattern         string `json:"filePattern,omitempty"`
	IncludeSourceColumn bool   `json:"includeSourceColumn,omitempty"`
}

// RemoteAnnotation represents an annotation from the sync API
type RemoteAnnotation struct {
	AnnotationID   string                  `json:"annotation_id"`
	WorkspaceID    string                  `json:"workspace_id"`
	FilePath       string                  `json:"file_path"`
	FileHash       string                  `json:"file_hash,omitempty"`
	Options        RemoteAnnotationOptions `json:"options,omitempty"`
	RowIndex       int                     `json:"row_index"`
	Content        string                  `json:"content"`
	AnnotationType string                  `json:"annotation_type"`
	Position       map[string]interface{}  `json:"position,omitempty"`
	ColumnHashes   []map[string]string     `json:"column_hashes,omitempty"`
	Note           string                  `json:"note,omitempty"`
	Color          string                  `json:"color,omitempty"`
	Tags           []string                `json:"tags,omitempty"`
	CreatedBy      string                  `json:"created_by"`
	CreatedByName  string                  `json:"created_by_name"`
	CreatedAt      string                  `json:"created_at"`
	UpdatedAt      string                  `json:"updated_at"`
	Version        int                     `json:"version"`
}

// SyncClient defines the interface for interacting with the sync API
// Methods return interface{} to avoid import cycles - the implementation will return compatible types
type SyncClient interface {
	IsLoggedIn() bool
	GetRemoteWorkspacesForClient() (interface{}, error)
	GetWorkspaceDetailsForClient(workspaceID string) (interface{}, error)
	GetWorkspaceAnnotationsForClient(workspaceID string) (interface{}, error)
	GetWorkspaceFilesForClient(workspaceID string) (interface{}, error)
	CreateFile(workspaceID, fileHash string, opts interfaces.FileOptions, description, filePath string) error
	CreateWorkspaceForClient(name string) (interface{}, error)
	DeleteFile(workspaceID, fileHash string, opts interfaces.FileOptions) error
	UpdateFileDescription(workspaceID, fileHash string, opts interfaces.FileOptions, description string) error
	GetFileLocationsForInstance() (interface{}, error)
	CreateAnnotation(workspaceID, fileHash string, opts interfaces.FileOptions, rowIndex int, note, color string) (string, error)
	CreateAnnotationBatch(workspaceID, fileHash string, opts interfaces.FileOptions, annotationRows []interface{}, note, color string) ([]string, error)
	UpdateAnnotation(workspaceID, annotationID, note, color string) error
	UpdateAnnotationBatch(workspaceID string, updates []api.UpdateAnnotationRequest) (*api.BatchUpdateAnnotationResponse, error)
	DeleteAnnotation(workspaceID, annotationID string) error
	DeleteAnnotationBatch(workspaceID string, annotationIDs []string) ([]string, error)
}

// RemoteWorkspaceService manages remote workspaces stored in the sync API
type RemoteWorkspaceService struct {
	ctx           context.Context
	app           interfaces.AppService
	syncClient    SyncClient
	workspaceID   string
	workspaceName string
	workspaceMu   sync.RWMutex

	// Cached workspace data
	annotationsMap map[string]map[int]*interfaces.RowAnnotation // composite_key -> row_index -> annotation
	fileData       map[string]*interfaces.WorkspaceFile         // composite_key -> file data
	hashKey        []byte                                       // HighwayHash key (32 bytes)

	// Annotation ID mapping for remote annotations (row_index -> annotation_id for API calls)
	annotationIDs map[string]string // annotation_id -> composite_key (for deletion lookups)

	// Periodic sync management
	syncTicker   *time.Ticker
	syncStop     chan bool
	syncInterval time.Duration
	syncMu       sync.Mutex
	isSync       bool // Flag to prevent sync during sync operations

	// Cache invalidation tracking
	lastKnownUpdatedAt string             // Track workspace UpdatedAt for change detection
	cacheInvalidators  []CacheInvalidator // List of registered cache invalidators
	invalidatorMu      sync.RWMutex       // Protect invalidator list
}

// NewRemoteWorkspaceService creates a new remote workspace service
func NewRemoteWorkspaceService(syncClient SyncClient) *RemoteWorkspaceService {
	return &RemoteWorkspaceService{
		syncClient:     syncClient,
		annotationsMap: make(map[string]map[int]*interfaces.RowAnnotation),
		fileData:       make(map[string]*interfaces.WorkspaceFile),
		annotationIDs:  make(map[string]string),
		syncInterval:   10 * time.Second, // Default 10-second sync interval
		syncStop:       make(chan bool, 1),
	}
}

// SetApp allows the main function to inject the App reference
func (rws *RemoteWorkspaceService) SetApp(app interfaces.AppService) {
	rws.app = app
}

// extractFileOptionsFromMap extracts FileOptions from the nested "options" object in sync-api responses
func extractFileOptionsFromMap(fileData map[string]interface{}) interfaces.FileOptions {
	var opts interfaces.FileOptions

	if optionsRaw, ok := fileData["options"]; ok {
		if optionsMap, ok := optionsRaw.(map[string]interface{}); ok {
			if jpath, ok := optionsMap["jpath"].(string); ok {
				opts.JPath = jpath
			}
			if noHeaderRow, ok := optionsMap["noHeaderRow"].(bool); ok {
				opts.NoHeaderRow = noHeaderRow
			}
			if ingestTz, ok := optionsMap["ingestTimezoneOverride"].(string); ok {
				opts.IngestTimezoneOverride = ingestTz
			}
			// Directory loading options
			if isDirectory, ok := optionsMap["isDirectory"].(bool); ok {
				opts.IsDirectory = isDirectory
			}
			if filePattern, ok := optionsMap["filePattern"].(string); ok {
				opts.FilePattern = filePattern
			}
			if includeSourceColumn, ok := optionsMap["includeSourceColumn"].(bool); ok {
				opts.IncludeSourceColumn = includeSourceColumn
			}
		}
	}

	return opts
}

// Startup receives the Wails context
func (rws *RemoteWorkspaceService) Startup(ctx context.Context) {
	rws.ctx = ctx
}

// CreateWorkspace creates a new remote workspace via the sync API
func (rws *RemoteWorkspaceService) CreateWorkspace(workspaceIdentifier string) error {
	if !rws.syncClient.IsLoggedIn() {
		return errors.New("not logged in to sync service")
	}

	// workspaceIdentifier is the workspace name for remote workspaces
	name := workspaceIdentifier
	if name == "" {
		return errors.New("workspace name is required")
	}

	// Create the workspace via sync API
	responseRaw, err := rws.syncClient.CreateWorkspaceForClient(name)
	if err != nil {
		return fmt.Errorf("failed to create remote workspace: %w", err)
	}

	// Convert interface{} to response data via JSON
	var response map[string]interface{}
	data, _ := json.Marshal(responseRaw)
	json.Unmarshal(data, &response)

	workspaceID, _ := response["workspace_id"].(string)
	if workspaceID == "" {
		return fmt.Errorf("invalid response from sync API: missing workspace_id")
	}

	rws.app.Log("info", fmt.Sprintf("Created remote workspace: %s (ID: %s)", name, workspaceID))
	return nil
}

// CreateWorkspaceAndReturnID creates a new remote workspace and returns the workspace ID
func (rws *RemoteWorkspaceService) CreateWorkspaceAndReturnID(name string) (string, error) {
	if !rws.syncClient.IsLoggedIn() {
		return "", errors.New("not logged in to sync service")
	}

	if name == "" {
		return "", errors.New("workspace name is required")
	}

	// Create the workspace via sync API
	responseRaw, err := rws.syncClient.CreateWorkspaceForClient(name)
	if err != nil {
		return "", fmt.Errorf("failed to create remote workspace: %w", err)
	}

	// Convert interface{} to response data via JSON
	var response map[string]interface{}
	data, _ := json.Marshal(responseRaw)
	json.Unmarshal(data, &response)

	workspaceID, _ := response["workspace_id"].(string)
	if workspaceID == "" {
		return "", fmt.Errorf("invalid response from sync API: missing workspace_id")
	}

	rws.app.Log("info", fmt.Sprintf("Created remote workspace: %s (ID: %s)", name, workspaceID))
	return workspaceID, nil
}

// ChooseAndOpenWorkspace opens a dialog to choose and open a remote workspace
func (rws *RemoteWorkspaceService) ChooseAndOpenWorkspace() error {
	if rws.ctx == nil {
		return errors.New("service not initialized")
	}

	// Check if licensed
	if !rws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	// Check if logged in to sync service
	if !rws.syncClient.IsLoggedIn() {
		return errors.New("not logged in to sync service")
	}

	// Get list of remote workspaces
	workspacesRaw, err := rws.syncClient.GetRemoteWorkspacesForClient()
	if err != nil {
		return fmt.Errorf("failed to get remote workspaces: %w", err)
	}

	// Convert interface{} to []RemoteWorkspace via JSON
	var workspaces []RemoteWorkspace
	data, _ := json.Marshal(workspacesRaw)
	json.Unmarshal(data, &workspaces)

	if len(workspaces) == 0 {
		return errors.New("no remote workspaces found")
	}

	// TODO: Show a dialog to select a workspace
	// For now, just open the first workspace
	if len(workspaces) > 0 {
		return rws.OpenWorkspace(workspaces[0].ID)
	}

	return errors.New("no workspaces available")
}

// OpenWorkspace opens a remote workspace by downloading it from the sync service
func (rws *RemoteWorkspaceService) OpenWorkspace(workspaceIdentifier string) error {
	if !rws.syncClient.IsLoggedIn() {
		return errors.New("not logged in to sync service")
	}

	// Get workspace details
	detailsRaw, err := rws.syncClient.GetWorkspaceDetailsForClient(workspaceIdentifier)
	if err != nil {
		return fmt.Errorf("failed to get workspace details: %w", err)
	}

	// Convert interface{} to RemoteWorkspaceDetails via JSON
	var details RemoteWorkspaceDetails
	data, _ := json.Marshal(detailsRaw)
	json.Unmarshal(data, &details)

	// Extract and decode hash key from workspace details
	var hashKey []byte
	if detailsMap, ok := detailsRaw.(map[string]interface{}); ok {
		if hk, exists := detailsMap["hash_key"]; exists {
			if hashKeyStr, ok := hk.(string); ok && hashKeyStr != "" {
				// Decode base64 hash key
				decoded, err := base64.StdEncoding.DecodeString(hashKeyStr)
				if err != nil {
					rws.app.Log("warning", fmt.Sprintf("Failed to decode workspace hash key: %v", err))
				} else {
					hashKey = decoded
				}
			}
		}
	}

	// Get workspace annotations
	annotationsRaw, err := rws.syncClient.GetWorkspaceAnnotationsForClient(workspaceIdentifier)
	if err != nil {
		return fmt.Errorf("failed to get workspace annotations: %w", err)
	}

	// Convert interface{} to []RemoteAnnotation via JSON
	var annotations []RemoteAnnotation
	data, _ = json.Marshal(annotationsRaw)

	// Debug: log the raw annotations JSON to see what options are present
	rws.app.Log("info", fmt.Sprintf("[WORKSPACE_LOAD] Raw annotations JSON: %s", string(data)))

	json.Unmarshal(data, &annotations)

	// Debug: log the parsed annotations to verify options
	for i, annot := range annotations {
		rws.app.Log("info", fmt.Sprintf("[WORKSPACE_LOAD] Parsed annotation %d: ID=%s, FileHash=%s, Options=%+v", i, annot.AnnotationID, annot.FileHash, annot.Options))
	}

	// Get all workspace files
	filesRaw, err := rws.syncClient.GetWorkspaceFilesForClient(workspaceIdentifier)
	if err != nil {
		return fmt.Errorf("failed to get workspace files: %w", err)
	}

	// Convert interface{} to []RemoteFile via JSON
	var files []map[string]interface{}
	data, _ = json.Marshal(filesRaw)

	// Debug: log the raw JSON to see what fields are present
	rws.app.Log("debug", fmt.Sprintf("[WORKSPACE_LOAD] Raw files JSON: %s", string(data)))

	json.Unmarshal(data, &files)

	// Load the workspace data into memory
	rws.workspaceMu.Lock()
	rws.workspaceID = workspaceIdentifier
	rws.workspaceName = details.Name
	rws.lastKnownUpdatedAt = details.UpdatedAt // Store initial timestamp
	rws.hashKey = hashKey                      // Store the decoded hash key
	rws.annotationsMap = make(map[string]map[int]*interfaces.RowAnnotation)
	rws.fileData = make(map[string]*interfaces.WorkspaceFile)
	rws.annotationIDs = make(map[string]string)

	// First, create file entries from all files in the workspace
	fileMap := make(map[string]*interfaces.WorkspaceFile)

	for _, fileData := range files {
		fileHash, _ := fileData["file_hash"].(string)
		description, _ := fileData["description"].(string)

		// Extract options from the nested "options" object (as returned by sync-api)
		opts := extractFileOptionsFromMap(fileData)

		// Debug: log the extracted options
		rws.app.Log("debug", fmt.Sprintf("[WORKSPACE_LOAD] File %s: options=%+v", fileHash, opts))
		compositeKey := makeCompositeKey(fileHash, opts)
		fileMap[compositeKey] = &interfaces.WorkspaceFile{
			FilePath:    "", // Will be set later from file locations table
			FileHash:    fileHash,
			Options:     opts,
			Description: description,
			Annotations: make([]interfaces.RowAnnotation, 0),
		}
	}

	// Then, process annotations and merge with file data
	// Only attach annotations to files that have actual file records - don't create ghost files from annotations
	rws.app.Log("info", fmt.Sprintf("[WORKSPACE_LOAD] Processing %d annotations", len(annotations)))
	for _, annot := range annotations {
		// Use the annotation's options to construct the composite key
		// This ensures annotations are correctly associated with files based on their settings
		annotOpts := interfaces.FileOptions{
			JPath:                  annot.Options.JPath,
			NoHeaderRow:            annot.Options.NoHeaderRow,
			IngestTimezoneOverride: annot.Options.IngestTimezoneOverride,
			// Directory loading options
			IsDirectory:         annot.Options.IsDirectory,
			FilePattern:         annot.Options.FilePattern,
			IncludeSourceColumn: annot.Options.IncludeSourceColumn,
		}
		compositeKey := makeCompositeKey(annot.FileHash, annotOpts)
		rws.app.Log("debug", fmt.Sprintf("[WORKSPACE_LOAD] Annotation %s: fileHash=%s, options=%+v -> compositeKey=%s",
			annot.AnnotationID, annot.FileHash, annot.Options, compositeKey))

		// Only attach annotations to existing file records - don't create ghost files
		if _, exists := fileMap[compositeKey]; !exists {
			rws.app.Log("debug", fmt.Sprintf("[WORKSPACE_LOAD] Skipping orphaned annotation %s - no file record exists for compositeKey=%s",
				annot.AnnotationID, compositeKey))
			continue
		}

		// Convert sync API annotation to local annotation format using the actual RowIndex from the API
		rowAnnotation := interfaces.RowAnnotation{
			AnnotationID: annot.AnnotationID,
			RowIndex:     annot.RowIndex,
			Note:         annot.Note,
			Color:        annot.Color,
		}

		fileMap[compositeKey].Annotations = append(fileMap[compositeKey].Annotations, rowAnnotation)

		// Build annotations map for fast lookup
		if rws.annotationsMap[compositeKey] == nil {
			rws.annotationsMap[compositeKey] = make(map[int]*interfaces.RowAnnotation)
		}

		annotCopy := rowAnnotation
		rws.annotationsMap[compositeKey][annot.RowIndex] = &annotCopy

		// Store annotation ID for deletion lookups
		rws.annotationIDs[annot.AnnotationID] = compositeKey

		rws.app.Log("debug", fmt.Sprintf("Loaded annotation %s for file %s at row index %d", annot.AnnotationID, compositeKey, annot.RowIndex))
	}

	// Get file locations for this instance and apply them to files
	fileLocationsRaw, err := rws.syncClient.GetFileLocationsForInstance()
	if err != nil {
		// Log warning but don't fail - file locations are optional
		rws.app.Log("warning", fmt.Sprintf("Failed to get file locations: %v", err))
	} else {
		// Convert interface{} to ListFileLocationsResponse first
		var response map[string]interface{}
		if locationsData, err := json.Marshal(fileLocationsRaw); err == nil {
			if err := json.Unmarshal(locationsData, &response); err == nil {
				// Extract the file_locations array from the response
				if fileLocationsArray, ok := response["file_locations"].([]interface{}); ok {
					rws.app.Log("info", fmt.Sprintf("Retrieved %d file locations from API", len(fileLocationsArray)))

					// Apply file locations to matching files
					for _, locationInterface := range fileLocationsArray {
						if location, ok := locationInterface.(map[string]interface{}); ok {
							fileHash, _ := location["file_hash"].(string)
							filePath, _ := location["file_path"].(string)
							workspaceID, _ := location["workspace_id"].(string)

							rws.app.Log("debug", fmt.Sprintf("Processing file location: hash=%s, path=%s, workspace=%s", fileHash, filePath, workspaceID))

							// Only apply locations for files in the current workspace
							rws.app.Log("debug", fmt.Sprintf("File location check: location.workspace=%s, current.workspace=%s, match=%v, hash=%s, path=%s",
								workspaceID, workspaceIdentifier, workspaceID == workspaceIdentifier, fileHash, filePath))
							if workspaceID == workspaceIdentifier && fileHash != "" && filePath != "" {
								// Find all files with this hash (there might be multiple jpaths)
								matchFound := false
								for _, file := range fileMap {
									if file.FileHash == fileHash {
										matchFound = true
										// Set the relative path to the local file location
										file.RelativePath = filePath
										// If FilePath is empty, also set it to have a fallback
										if file.FilePath == "" {
											file.FilePath = filePath
										}
										rws.app.Log("info", fmt.Sprintf("Applied file location: %s -> %s", fileHash, filePath))
									}
								}
								if !matchFound {
									rws.app.Log("debug", fmt.Sprintf("No matching file found for hash: %s", fileHash))
								}
							} else {
								rws.app.Log("debug", fmt.Sprintf("Skipping file location: workspace mismatch (got=%s, want=%s) or empty data (hash=%s, path=%s)",
									workspaceID, workspaceIdentifier, fileHash, filePath))
							}
						}
					}
				} else {
					rws.app.Log("warning", "No file_locations array found in API response")
				}
			} else {
				rws.app.Log("warning", fmt.Sprintf("Failed to unmarshal file locations response: %v", err))
			}
		} else {
			rws.app.Log("warning", fmt.Sprintf("Failed to marshal file locations data: %v", err))
		}
	}

	// Store file data
	for key, file := range fileMap {
		rws.fileData[key] = file
	}

	rws.workspaceMu.Unlock()

	// Update window title
	rws.updateWindowTitle()

	// Start periodic sync
	rws.startPeriodicSync()

	rws.app.Log("info", fmt.Sprintf("Remote workspace loaded: %s", details.Name))
	return nil
}

// CloseWorkspace closes the active remote workspace
func (rws *RemoteWorkspaceService) CloseWorkspace() error {
	if rws.ctx == nil {
		return errors.New("service not initialized")
	}

	// Stop periodic sync
	rws.stopPeriodicSync()

	rws.workspaceMu.Lock()
	rws.workspaceID = ""
	rws.workspaceName = ""
	rws.hashKey = nil // Clear the hash key
	rws.annotationsMap = make(map[string]map[int]*interfaces.RowAnnotation)
	rws.fileData = make(map[string]*interfaces.WorkspaceFile)
	rws.annotationIDs = make(map[string]string)
	rws.workspaceMu.Unlock()

	// Update window title to default
	rws.updateWindowTitle()

	// Invalidate annotation caches to clear old workspace data
	rws.invalidateAnnotationCaches()

	rws.app.Log("info", "Remote workspace closed")
	return nil
}

// AddAnnotations adds annotations to a remote workspace (syncs to server)
func (rws *RemoteWorkspaceService) AddAnnotations(fileHash string, opts interfaces.FileOptions, rowIndices []int, timeField string, note string, color string, query string) error {
	rws.app.Log("info", fmt.Sprintf("[ADD_ANNOTATIONS] Called with fileHash=%s, opts=%+v, rowIndices=%v",
		fileHash, opts, rowIndices))

	if !rws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	rws.workspaceMu.RLock()
	hasWorkspace := rws.workspaceID != ""
	workspaceID := rws.workspaceID
	rws.workspaceMu.RUnlock()

	if !hasWorkspace {
		return errors.New("no remote workspace is open")
	}

	if !rws.syncClient.IsLoggedIn() {
		return errors.New("not logged in to sync service")
	}

	// Check if file is in workspace, add it if not
	files, err := rws.GetWorkspaceFiles()
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

	tab := rws.app.GetActiveTab()
	if tab == nil {
		return errors.New("file not open in active tab")
	}

	if workspaceFile == nil {
		// This file isn't in the workspace, add it so that we can annotate it
		// Add file to workspace before annotating
		if err := rws.AddFileToWorkspace(fileHash, opts, tab.FilePath, ""); err != nil {
			return fmt.Errorf("failed to add file to workspace: %w", err)
		}
		// Update workspaceFile reference after adding
		workspaceFile = &interfaces.WorkspaceFile{
			FilePath:    tab.FilePath,
			FileHash:    fileHash,
			Options:     opts,
			Description: "",
		}
	}

	// Execute query - returns FULL rows now!
	result, err := rws.app.ExecuteQueryForTabWithMetadata(tab, query, timeField)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	// Get original header from query result
	originalHeader := result.OriginalHeader
	if len(originalHeader) == 0 {
		return fmt.Errorf("missing original header in query result")
	}

	// CRITICAL: Use StageResult.Rows which have the correct RowIndex from the original file
	// The rowIndices parameter contains DISPLAY indices (0, 1, 2...) which may differ from
	// the actual file row indices when data is sorted/filtered
	if result.StageResult == nil || len(result.StageResult.Rows) == 0 {
		return fmt.Errorf("no stage result available for annotation creation")
	}

	stageRows := result.StageResult.Rows

	// Default to grey if no color specified
	if color == "" {
		color = "grey"
	}

	// Process each row index and separate into updates and new annotations
	var newAnnotationRows []interface{}
	var updateOperations []struct {
		annotationID string
		rowIndex     int
	}
	// Track which rows were included in batch creation (for correct ID mapping)
	var batchCreationRows []struct {
		rowIndex int
	}

	compositeKey := makeCompositeKey(fileHash, opts)

	rws.app.Log("debug", fmt.Sprintf("Processing annotation for %d display indices, total rows in result: %d", len(rowIndices), len(stageRows)))

	// Process display indices and map to actual file row indices
	for _, displayIndex := range rowIndices {
		if displayIndex < 0 || displayIndex >= len(stageRows) {
			continue // Skip invalid indices
		}

		// Get actual file row index from StageResult
		actualRowIndex := stageRows[displayIndex].RowIndex
		rws.app.Log("debug", fmt.Sprintf("Display index %d maps to file row index %d", displayIndex, actualRowIndex))

		// Check if annotation already exists for this actual row index
		rws.workspaceMu.RLock()
		fileAnnots := rws.annotationsMap[compositeKey]
		var existingAnnotation *interfaces.RowAnnotation
		if fileAnnots != nil {
			existingAnnotation = fileAnnots[actualRowIndex]
		}
		rws.workspaceMu.RUnlock()

		if existingAnnotation != nil {
			// Queue for update using actual row index
			rws.app.Log("debug", fmt.Sprintf("Queueing annotation update for actual row %d (display %d), ID: %s", actualRowIndex, displayIndex, existingAnnotation.AnnotationID))
			updateOperations = append(updateOperations, struct {
				annotationID string
				rowIndex     int
			}{existingAnnotation.AnnotationID, actualRowIndex})
		} else {
			// Queue for batch creation with actual RowIndex
			rws.app.Log("debug", fmt.Sprintf("Queueing new annotation for actual row %d (display %d)", actualRowIndex, displayIndex))
			annotationRow := api.AnnotationRow{
				RowIndex: actualRowIndex,
			}
			newAnnotationRows = append(newAnnotationRows, annotationRow)

			// Track this row for correct ID mapping later
			batchCreationRows = append(batchCreationRows, struct {
				rowIndex int
			}{actualRowIndex})
		}
	}

	// Process updates in batches of 256
	successCount := 0
	var successfulUpdates []struct {
		annotationID string
		rowIndex     int
	}
	if len(updateOperations) > 0 {
		const maxBatchSize = 256
		totalUpdates := len(updateOperations)

		rws.app.Log("info", fmt.Sprintf("Processing %d annotation updates in batches of %d", totalUpdates, maxBatchSize))

		for i := 0; i < totalUpdates; i += maxBatchSize {
			// Calculate batch boundaries
			endIdx := i + maxBatchSize
			if endIdx > totalUpdates {
				endIdx = totalUpdates
			}

			batchUpdates := updateOperations[i:endIdx]
			batchNum := (i / maxBatchSize) + 1
			totalBatches := (totalUpdates + maxBatchSize - 1) / maxBatchSize

			rws.app.Log("info", fmt.Sprintf("Sending update batch %d/%d with %d updates", batchNum, totalBatches, len(batchUpdates)))

			// Convert to API update requests
			var apiUpdates []api.UpdateAnnotationRequest
			for _, update := range batchUpdates {
				apiUpdates = append(apiUpdates, api.UpdateAnnotationRequest{
					AnnotationID: update.annotationID,
					Note:         note,
					Color:        api.AnnotationColor(color),
				})
			}

			// Call batch update API
			response, err := rws.syncClient.UpdateAnnotationBatch(workspaceID, apiUpdates)
			if err != nil {
				rws.app.Log("error", fmt.Sprintf("Failed to update annotation batch %d/%d: %v", batchNum, totalBatches, err))

				// Fallback to individual updates for this batch
				rws.app.Log("info", fmt.Sprintf("Falling back to individual updates for batch %d/%d", batchNum, totalBatches))
				for _, update := range batchUpdates {
					err := rws.syncClient.UpdateAnnotation(workspaceID, update.annotationID, note, color)
					if err != nil {
						rws.app.Log("error", fmt.Sprintf("Failed to update annotation %s for row %d: %v", update.annotationID, update.rowIndex, err))
						continue
					}
					rws.app.Log("info", fmt.Sprintf("Updated annotation %s for row %d", update.annotationID, update.rowIndex))
					successCount++
					successfulUpdates = append(successfulUpdates, update)
				}
				continue
			}

			// Track successful updates from batch response
			if response != nil {
				successCount += response.SuccessCount
				// Only add to successful updates if batch succeeded
				if response.SuccessCount == len(batchUpdates) {
					successfulUpdates = append(successfulUpdates, batchUpdates...)
				}
				rws.app.Log("info", fmt.Sprintf("Successfully updated batch %d/%d: %d successful, %d failed",
					batchNum, totalBatches, response.SuccessCount, response.FailureCount))
			} else {
				// Assume all succeeded if no response details
				successCount += len(batchUpdates)
				successfulUpdates = append(successfulUpdates, batchUpdates...)
				rws.app.Log("info", fmt.Sprintf("Successfully updated batch %d/%d with %d updates", batchNum, totalBatches, len(batchUpdates)))
			}
		}

		rws.app.Log("info", fmt.Sprintf("Completed all update batches: %d/%d annotations updated successfully", successCount, totalUpdates))
	}

	// Process new annotations in batches of 256
	var allBatchAnnotationIDs []string
	var allBatchRowMappings []struct {
		rowIndex int
	}

	if len(newAnnotationRows) > 0 {
		const maxBatchSize = 256
		totalNewAnnotations := len(newAnnotationRows)

		rws.app.Log("info", fmt.Sprintf("Processing %d new annotations in batches of %d", totalNewAnnotations, maxBatchSize))

		for i := 0; i < totalNewAnnotations; i += maxBatchSize {
			// Calculate batch boundaries
			endIdx := i + maxBatchSize
			if endIdx > totalNewAnnotations {
				endIdx = totalNewAnnotations
			}

			batchRows := newAnnotationRows[i:endIdx]
			batchCreationRowsSlice := batchCreationRows[i:endIdx]
			batchNum := (i / maxBatchSize) + 1
			totalBatches := (totalNewAnnotations + maxBatchSize - 1) / maxBatchSize

			rws.app.Log("info", fmt.Sprintf("Sending batch %d/%d with %d annotations", batchNum, totalBatches, len(batchRows)))

			annotationIDs, err := rws.syncClient.CreateAnnotationBatch(workspaceID, fileHash, opts, batchRows, note, color)
			if err != nil {
				rws.app.Log("error", fmt.Sprintf("Failed to create annotation batch %d/%d: %v", batchNum, totalBatches, err))
				continue // Continue with next batch even if this one fails
			}

			// Store the annotation IDs and their corresponding row mappings
			allBatchAnnotationIDs = append(allBatchAnnotationIDs, annotationIDs...)
			allBatchRowMappings = append(allBatchRowMappings, batchCreationRowsSlice...)

			rws.app.Log("info", fmt.Sprintf("Successfully created batch %d/%d with %d annotations", batchNum, totalBatches, len(annotationIDs)))
		}

		rws.app.Log("info", fmt.Sprintf("Completed all batches: %d/%d annotations created successfully", len(allBatchAnnotationIDs), totalNewAnnotations))
	}

	// Update local cache for all successful annotations
	rws.workspaceMu.Lock()
	if rws.annotationsMap[compositeKey] == nil {
		rws.annotationsMap[compositeKey] = make(map[int]*interfaces.RowAnnotation)
	}

	// Calculate total attempted and success counts
	totalAttempted := len(updateOperations) + len(newAnnotationRows)
	totalSuccessCount := successCount + len(allBatchAnnotationIDs)
	totalFailedCount := totalAttempted - totalSuccessCount

	if totalSuccessCount == 0 {
		rws.workspaceMu.Unlock()
		return fmt.Errorf("annotation_update_failed: %d of %d annotation(s) failed to save", totalFailedCount, totalAttempted)
	}

	// If some but not all failed, still return an error with partial success info
	if totalFailedCount > 0 {
		rws.app.Log("warn", fmt.Sprintf("Partial annotation failure: %d of %d failed", totalFailedCount, totalAttempted))
	}

	// Process both updated and newly created annotations for cache updates
	allProcessedRows := make([]struct {
		rowIndex     int
		annotationID string
	}, 0)

	// Add only SUCCESSFUL update operations
	for _, update := range successfulUpdates {
		allProcessedRows = append(allProcessedRows, struct {
			rowIndex     int
			annotationID string
		}{update.rowIndex, update.annotationID})
	}

	// Add batch created annotations using the tracked rows
	for batchIndex, batchRow := range allBatchRowMappings {
		if batchIndex < len(allBatchAnnotationIDs) {
			// This was a new annotation that was created in batch
			annotationID := allBatchAnnotationIDs[batchIndex]
			allProcessedRows = append(allProcessedRows, struct {
				rowIndex     int
				annotationID string
			}{batchRow.rowIndex, annotationID})
		}
	}

	// Update cache for all processed annotations using row index
	for _, processed := range allProcessedRows {
		rowIndex := processed.rowIndex
		annotationID := processed.annotationID

		// Create annotation for local cache
		annotation := &interfaces.RowAnnotation{
			AnnotationID: annotationID,
			RowIndex:     rowIndex,
			Note:         note,
			Color:        color,
		}

		// Store annotation by row index (O(1) update/insert)
		rws.annotationsMap[compositeKey][rowIndex] = annotation

		// Store annotation ID for deletion lookups
		rws.annotationIDs[annotationID] = compositeKey
	}
	rws.workspaceMu.Unlock()

	// After successful annotation addition and before emitting workspace event
	rws.invalidateAnnotationCaches()

	// Emit workspace updated event to notify frontend
	if rws.ctx != nil {
		runtime.EventsEmit(rws.ctx, "workspace:updated")
	}

	rws.app.Log("info", fmt.Sprintf("Annotation added to %d row(s) in remote workspace", totalSuccessCount))
	return nil
}

// AddAnnotationsWithRows adds annotations to a remote workspace using pre-fetched row data
// This is an optimized version that avoids redundant query execution
func (rws *RemoteWorkspaceService) AddAnnotationsWithRows(fileHash string, opts interfaces.FileOptions, rowIndices []int, rows [][]string, header []string, timeField string, note string, color string, query string) error {
	if !rws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	rws.workspaceMu.RLock()
	hasWorkspace := rws.workspaceID != ""
	workspaceID := rws.workspaceID
	rws.workspaceMu.RUnlock()

	if !hasWorkspace {
		return errors.New("no remote workspace is open")
	}

	if !rws.syncClient.IsLoggedIn() {
		return errors.New("not logged in to sync service")
	}

	// Check if file is in workspace, add it if not
	files, err := rws.GetWorkspaceFiles()
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

	tab := rws.app.GetActiveTab()
	if tab == nil {
		return errors.New("file not open in active tab")
	}

	if workspaceFile == nil {
		// This file isn't in the workspace, add it so that we can annotate it
		// Add file to workspace before annotating
		if err := rws.AddFileToWorkspace(fileHash, opts, tab.FilePath, ""); err != nil {
			return fmt.Errorf("failed to add file to workspace: %w", err)
		}
		// Update workspaceFile reference after adding
		workspaceFile = &interfaces.WorkspaceFile{
			FilePath:    tab.FilePath,
			FileHash:    fileHash,
			Options:     opts,
			Description: "",
		}
	}

	// Use provided rows (no need to re-query)
	queryRows := rows
	_ = header // header is no longer needed for row-index-based annotations

	// Default to grey if no color specified
	if color == "" {
		color = "grey"
	}

	// Process each row index and separate into updates and new annotations
	var newAnnotationRows []interface{}
	var updateOperations []struct {
		annotationID string
		rowIndex     int
	}
	// Track which rows were included in batch creation (for correct ID mapping)
	var batchCreationRows []struct {
		rowIndex int
	}

	compositeKey := makeCompositeKey(fileHash, opts)

	// Process rows using row-index-based lookup
	// rowIndices[i] contains the original file row index for rows[i]
	// These arrays are parallel - validate using array index, not row index value
	for i, rowIndex := range rowIndices {
		if rowIndex < 0 || i >= len(queryRows) {
			continue // Skip invalid entries
		}

		// Check if annotation already exists for this row index
		rws.workspaceMu.RLock()
		fileAnnots := rws.annotationsMap[compositeKey]
		var existingAnnotation *interfaces.RowAnnotation
		if fileAnnots != nil {
			existingAnnotation = fileAnnots[rowIndex]
		}
		rws.workspaceMu.RUnlock()

		if existingAnnotation != nil {
			// Queue for update
			updateOperations = append(updateOperations, struct {
				annotationID string
				rowIndex     int
			}{existingAnnotation.AnnotationID, rowIndex})
		} else {
			// Queue for batch creation with RowIndex
			annotationRow := api.AnnotationRow{
				RowIndex: rowIndex,
			}
			newAnnotationRows = append(newAnnotationRows, annotationRow)

			// Track this row for correct ID mapping later
			batchCreationRows = append(batchCreationRows, struct {
				rowIndex int
			}{rowIndex})
		}
	}

	// Process updates in batches of 256
	successCount := 0
	var successfulUpdates []struct {
		annotationID string
		rowIndex     int
	}
	if len(updateOperations) > 0 {
		const maxBatchSize = 256
		totalUpdates := len(updateOperations)

		for i := 0; i < totalUpdates; i += maxBatchSize {
			endIdx := i + maxBatchSize
			if endIdx > totalUpdates {
				endIdx = totalUpdates
			}

			batchUpdates := updateOperations[i:endIdx]

			// Convert to API update requests
			var apiUpdates []api.UpdateAnnotationRequest
			for _, update := range batchUpdates {
				apiUpdates = append(apiUpdates, api.UpdateAnnotationRequest{
					AnnotationID: update.annotationID,
					Note:         note,
					Color:        api.AnnotationColor(color),
				})
			}

			// Call batch update API
			response, err := rws.syncClient.UpdateAnnotationBatch(workspaceID, apiUpdates)
			if err != nil {
				// Fallback to individual updates for this batch
				for _, update := range batchUpdates {
					err := rws.syncClient.UpdateAnnotation(workspaceID, update.annotationID, note, color)
					if err != nil {
						continue
					}
					successCount++
					successfulUpdates = append(successfulUpdates, update)
				}
				continue
			}

			// Track successful updates from batch response
			if response != nil {
				successCount += response.SuccessCount
				if response.SuccessCount == len(batchUpdates) {
					successfulUpdates = append(successfulUpdates, batchUpdates...)
				}
			} else {
				successCount += len(batchUpdates)
				successfulUpdates = append(successfulUpdates, batchUpdates...)
			}
		}
	}

	// Process new annotations in batches of 256
	var allBatchAnnotationIDs []string
	var allBatchRowMappings []struct {
		rowIndex int
	}

	if len(newAnnotationRows) > 0 {
		const maxBatchSize = 256
		totalNewAnnotations := len(newAnnotationRows)

		for i := 0; i < totalNewAnnotations; i += maxBatchSize {
			endIdx := i + maxBatchSize
			if endIdx > totalNewAnnotations {
				endIdx = totalNewAnnotations
			}

			batchRows := newAnnotationRows[i:endIdx]
			batchCreationRowsSlice := batchCreationRows[i:endIdx]

			annotationIDs, err := rws.syncClient.CreateAnnotationBatch(workspaceID, fileHash, opts, batchRows, note, color)
			if err != nil {
				continue // Continue with next batch even if this one fails
			}

			// Store the annotation IDs and their corresponding row mappings
			allBatchAnnotationIDs = append(allBatchAnnotationIDs, annotationIDs...)
			allBatchRowMappings = append(allBatchRowMappings, batchCreationRowsSlice...)
		}
	}

	// Update local cache for all successful annotations
	rws.workspaceMu.Lock()
	if rws.annotationsMap[compositeKey] == nil {
		rws.annotationsMap[compositeKey] = make(map[int]*interfaces.RowAnnotation)
	}

	// Calculate total attempted and success counts
	totalAttempted := len(updateOperations) + len(newAnnotationRows)
	totalSuccessCount := successCount + len(allBatchAnnotationIDs)
	totalFailedCount := totalAttempted - totalSuccessCount

	if totalSuccessCount == 0 {
		rws.workspaceMu.Unlock()
		return fmt.Errorf("annotation_update_failed: %d of %d annotation(s) failed to save", totalFailedCount, totalAttempted)
	}

	// If some but not all failed, log a warning
	if totalFailedCount > 0 {
		rws.app.Log("warn", fmt.Sprintf("Partial annotation failure: %d of %d failed", totalFailedCount, totalAttempted))
	}

	// Process both updated and newly created annotations for cache updates
	allProcessedRows := make([]struct {
		rowIndex     int
		annotationID string
	}, 0)

	// Add only SUCCESSFUL update operations
	for _, update := range successfulUpdates {
		allProcessedRows = append(allProcessedRows, struct {
			rowIndex     int
			annotationID string
		}{update.rowIndex, update.annotationID})
	}

	// Add batch created annotations using the tracked rows
	for batchIndex, batchRow := range allBatchRowMappings {
		if batchIndex < len(allBatchAnnotationIDs) {
			annotationID := allBatchAnnotationIDs[batchIndex]
			allProcessedRows = append(allProcessedRows, struct {
				rowIndex     int
				annotationID string
			}{batchRow.rowIndex, annotationID})
		}
	}

	// Update cache for all processed annotations using row index
	for _, processed := range allProcessedRows {
		rowIndex := processed.rowIndex
		annotationID := processed.annotationID

		// Create annotation for local cache
		annotation := &interfaces.RowAnnotation{
			AnnotationID: annotationID,
			RowIndex:     rowIndex,
			Note:         note,
			Color:        color,
		}

		// Store annotation by row index (O(1) update/insert)
		rws.annotationsMap[compositeKey][rowIndex] = annotation

		// Store annotation ID for deletion lookups
		rws.annotationIDs[annotationID] = compositeKey
	}
	rws.workspaceMu.Unlock()

	// After successful annotation addition and before emitting workspace event
	rws.invalidateAnnotationCaches()

	// Emit workspace updated event to notify frontend
	if rws.ctx != nil {
		runtime.EventsEmit(rws.ctx, "workspace:updated")
	}

	rws.app.Log("info", fmt.Sprintf("Annotation added to %d row(s) in remote workspace", totalSuccessCount))
	return nil
}

// IsRowAnnotatedByIndex checks if a row at a specific index has an annotation
// This is the preferred method for row-index-based annotation lookup
func (rws *RemoteWorkspaceService) IsRowAnnotatedByIndex(fileHash string, opts interfaces.FileOptions, rowIndex int) (bool, *interfaces.AnnotationResult) {
	rws.workspaceMu.RLock()
	defer rws.workspaceMu.RUnlock()

	// Check if we have annotations for this file+opts combination
	compositeKey := makeCompositeKey(fileHash, opts)
	fileAnnots, ok := rws.annotationsMap[compositeKey]
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

// GetRowAnnotation retrieves the annotation for a specific row
func (rws *RemoteWorkspaceService) GetRowAnnotation(fileHash string, opts interfaces.FileOptions, rowIndex int, query string, timeField string) (*interfaces.AnnotationResult, error) {
	workspaceFile, err := rws.GetWorkspaceFile(fileHash, opts)
	if workspaceFile == nil || err != nil {
		return nil, errors.New("file not in workspace")
	}

	tab := rws.app.GetActiveTab()
	if tab == nil || tab.FilePath != workspaceFile.FilePath {
		return nil, errors.New("file not open in active tab")
	}

	// Execute query - returns FULL rows with metadata
	result, err := rws.app.ExecuteQueryForTabWithMetadata(tab, query, timeField)
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
	annotated, annotResult := rws.IsRowAnnotatedByIndex(fileHash, opts, actualRowIndex)
	if annotated {
		return annotResult, nil
	}
	return &interfaces.AnnotationResult{Note: "", Color: ""}, nil
}

// GetRowAnnotations retrieves annotations for multiple rows
func (rws *RemoteWorkspaceService) GetRowAnnotations(fileHash string, opts interfaces.FileOptions, rowIndices []int, query string, timeField string) (map[int]*interfaces.AnnotationResult, error) {
	workspaceFile, err := rws.GetWorkspaceFile(fileHash, opts)
	if workspaceFile == nil || err != nil {
		return nil, errors.New("file not in workspace")
	}

	tab := rws.app.GetActiveTab()
	if tab == nil || tab.FilePath != workspaceFile.FilePath {
		return nil, errors.New("file not open in active tab")
	}

	// Execute query - returns FULL rows with metadata
	result, err := rws.app.ExecuteQueryForTabWithMetadata(tab, query, timeField)
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
		annotated, annotResult := rws.IsRowAnnotatedByIndex(fileHash, opts, originalRowIndex)
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
func (rws *RemoteWorkspaceService) GetRowAnnotationsWithRows(fileHash string, opts interfaces.FileOptions, rowIndices []int, rows [][]string, header []string) (map[int]*interfaces.AnnotationResult, error) {
	workspaceFile, err := rws.GetWorkspaceFile(fileHash, opts)
	if workspaceFile == nil || err != nil {
		return nil, errors.New("file not in workspace")
	}

	results := make(map[int]*interfaces.AnnotationResult)

	// Early exit if no annotations for this file
	if !rws.HasAnnotationsForFile(fileHash, opts) {
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
	hashKey := rws.GetHashKey()
	if hashKey == nil {
		return nil, errors.New("workspace hash key not available")
	}

	// Single batch check (parallel processing) to determine which rows are annotated
	annotatedFlags := rws.IsRowAnnotatedBatch(fileHash, opts, interfaceRows, hashKey)

	// Only fetch annotation details for rows that are annotated
	for i, rowIndex := range rowIndices {
		if i >= len(rows) {
			results[rowIndex] = &interfaces.AnnotationResult{Note: "", Color: ""}
			continue
		}

		if annotatedFlags[i] {
			// Use IsRowAnnotatedByIndex for proper row-index-based lookup
			// rowIndex contains the actual file row index (passed by caller)
			_, annotResult := rws.IsRowAnnotatedByIndex(fileHash, opts, rowIndex)
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

// DeleteRowAnnotations removes annotations from a remote workspace (syncs to server)
func (rws *RemoteWorkspaceService) DeleteRowAnnotations(fileHash string, opts interfaces.FileOptions, rowIndices []int, timeField string, query string) error {
	if !rws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	rws.workspaceMu.RLock()
	hasWorkspace := rws.workspaceID != ""
	workspaceID := rws.workspaceID
	rws.workspaceMu.RUnlock()

	if !hasWorkspace {
		return errors.New("no remote workspace is open")
	}

	if !rws.syncClient.IsLoggedIn() {
		return errors.New("not logged in to sync service")
	}

	workspaceFile, fileErr := rws.GetWorkspaceFile(fileHash, opts)
	if workspaceFile == nil || fileErr != nil {
		return errors.New("file not in workspace")
	}

	tab := rws.app.GetActiveTab()
	if tab == nil || tab.FilePath != workspaceFile.FilePath {
		return errors.New("file not open in active tab")
	}

	// Execute query - returns StageResult with Rows that have correct RowIndex
	result, err := rws.app.ExecuteQueryForTabWithMetadata(tab, query, timeField)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	// CRITICAL: Use StageResult.Rows which have the correct RowIndex from the original file
	// The rowIndices parameter contains DISPLAY indices (0, 1, 2...) which may differ from
	// the actual file row indices when data is sorted/filtered
	if result.StageResult == nil || len(result.StageResult.Rows) == 0 {
		return fmt.Errorf("no stage result available for annotation deletion")
	}

	stageRows := result.StageResult.Rows

	// First pass: collect all annotation IDs to delete using actual file row indices
	type annotationToDelete struct {
		annotationID string
		rowIndex     int
	}

	compositeKey := makeCompositeKey(fileHash, opts)
	var annotationsToDelete []annotationToDelete

	rws.workspaceMu.RLock()
	fileAnnots := rws.annotationsMap[compositeKey]
	rws.workspaceMu.RUnlock()

	rws.app.Log("debug", fmt.Sprintf("Processing deletion for %d display indices, total rows in result: %d", len(rowIndices), len(stageRows)))

	for _, displayIndex := range rowIndices {
		if displayIndex < 0 || displayIndex >= len(stageRows) {
			continue // Skip invalid indices
		}

		// Get actual file row index from StageResult
		actualRowIndex := stageRows[displayIndex].RowIndex
		rws.app.Log("debug", fmt.Sprintf("Display index %d maps to file row index %d", displayIndex, actualRowIndex))

		// Look up annotation by actual file row index (O(1))
		if fileAnnots == nil {
			rws.app.Log("warning", fmt.Sprintf("No annotations found for file %s", compositeKey))
			continue
		}

		annotation := fileAnnots[actualRowIndex]
		if annotation == nil {
			rws.app.Log("warning", fmt.Sprintf("No annotation found for actual row index %d (display index %d)", actualRowIndex, displayIndex))
			continue
		}

		annotationsToDelete = append(annotationsToDelete, annotationToDelete{
			annotationID: annotation.AnnotationID,
			rowIndex:     actualRowIndex,
		})
	}

	if len(annotationsToDelete) == 0 {
		return errors.New("no valid annotations found to delete")
	}

	// Process deletions in batches of 256 (same as creation)
	var deletedIDs []string
	const maxBatchSize = 256
	totalAnnotations := len(annotationsToDelete)

	if totalAnnotations == 1 {
		// Single annotation - use individual delete
		annotation := annotationsToDelete[0]
		deleteErr := rws.syncClient.DeleteAnnotation(workspaceID, annotation.annotationID)
		if deleteErr != nil {
			rws.app.Log("error", fmt.Sprintf("Failed to delete annotation %s for row %d: %v", annotation.annotationID, annotation.rowIndex, deleteErr))
		} else {
			deletedIDs = []string{annotation.annotationID}
		}
	} else {
		// Multiple annotations - process in batches
		rws.app.Log("info", fmt.Sprintf("Processing %d annotation deletions in batches of %d", totalAnnotations, maxBatchSize))

		for i := 0; i < totalAnnotations; i += maxBatchSize {
			// Calculate batch boundaries
			endIdx := i + maxBatchSize
			if endIdx > totalAnnotations {
				endIdx = totalAnnotations
			}

			batchAnnotations := annotationsToDelete[i:endIdx]
			batchNum := (i / maxBatchSize) + 1
			totalBatches := (totalAnnotations + maxBatchSize - 1) / maxBatchSize

			rws.app.Log("info", fmt.Sprintf("Sending delete batch %d/%d with %d annotations", batchNum, totalBatches, len(batchAnnotations)))

			// Extract annotation IDs for this batch
			batchAnnotationIDs := make([]string, len(batchAnnotations))
			for j, annotation := range batchAnnotations {
				batchAnnotationIDs[j] = annotation.annotationID
			}

			// Try batch delete first
			batchDeletedIDs, deleteErr := rws.syncClient.DeleteAnnotationBatch(workspaceID, batchAnnotationIDs)
			if deleteErr != nil {
				rws.app.Log("error", fmt.Sprintf("Failed to delete batch %d/%d: %v", batchNum, totalBatches, deleteErr))
				// Fall back to individual deletions for this batch
				rws.app.Log("info", fmt.Sprintf("Falling back to individual deletions for batch %d/%d", batchNum, totalBatches))
				for _, annotation := range batchAnnotations {
					fallbackErr := rws.syncClient.DeleteAnnotation(workspaceID, annotation.annotationID)
					if fallbackErr != nil {
						rws.app.Log("error", fmt.Sprintf("Failed to delete annotation %s for row %d: %v", annotation.annotationID, annotation.rowIndex, fallbackErr))
					} else {
						deletedIDs = append(deletedIDs, annotation.annotationID)
					}
				}
			} else {
				deletedIDs = append(deletedIDs, batchDeletedIDs...)
				rws.app.Log("info", fmt.Sprintf("Successfully deleted batch %d/%d with %d annotations", batchNum, totalBatches, len(batchDeletedIDs)))
			}
		}

		rws.app.Log("info", fmt.Sprintf("Completed all delete batches: %d/%d annotations deleted successfully", len(deletedIDs), totalAnnotations))
	}

	if len(deletedIDs) == 0 {
		return errors.New("no annotations could be deleted")
	}

	// Update local cache for successfully deleted annotations
	deleteCount := 0
	deletedIDsMap := make(map[string]bool)
	for _, id := range deletedIDs {
		deletedIDsMap[id] = true
	}

	rws.workspaceMu.Lock()
	for _, annotation := range annotationsToDelete {
		if !deletedIDsMap[annotation.annotationID] {
			continue // Skip annotations that failed to delete
		}

		// Remove from local cache using row index (O(1) deletion)
		if rws.annotationsMap[compositeKey] != nil {
			delete(rws.annotationsMap[compositeKey], annotation.rowIndex)
		}

		// Remove annotation ID mapping
		delete(rws.annotationIDs, annotation.annotationID)

		rws.app.Log("info", fmt.Sprintf("Deleted annotation %s for row %d", annotation.annotationID, annotation.rowIndex))
		deleteCount++
	}
	rws.workspaceMu.Unlock()

	if deleteCount == 0 {
		return errors.New("no annotations were successfully deleted")
	}

	// After successful annotation deletion and before emitting workspace event
	rws.invalidateAnnotationCaches()

	// Emit workspace updated event to notify frontend
	if rws.ctx != nil {
		runtime.EventsEmit(rws.ctx, "workspace:updated")
	}

	rws.app.Log("info", fmt.Sprintf("Annotation deleted for %d row(s) in remote workspace", deleteCount))
	return nil
}

// DeleteRowAnnotationsWithRows removes annotations from a remote workspace using pre-fetched row data
// This is an optimized version that avoids redundant query execution
// NOTE: rowIndices should contain actual file row indices (not display indices)
// The rows array should be positionally aligned with rowIndices (rows[i] corresponds to rowIndices[i])
func (rws *RemoteWorkspaceService) DeleteRowAnnotationsWithRows(fileHash string, opts interfaces.FileOptions, rowIndices []int, rows [][]string, header []string, timeField string, query string) error {
	if !rws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	rws.workspaceMu.RLock()
	hasWorkspace := rws.workspaceID != ""
	workspaceID := rws.workspaceID
	rws.workspaceMu.RUnlock()

	if !hasWorkspace {
		return errors.New("no remote workspace is open")
	}

	if !rws.syncClient.IsLoggedIn() {
		return errors.New("not logged in to sync service")
	}

	workspaceFile, fileErr := rws.GetWorkspaceFile(fileHash, opts)
	if workspaceFile == nil || fileErr != nil {
		return errors.New("file not in workspace")
	}

	_ = header // header is no longer needed for row-index-based annotations

	rws.app.Log("debug", fmt.Sprintf("Processing deletion for %d row indices with pre-fetched rows", len(rowIndices)))

	// First pass: collect all annotation IDs to delete using actual file row indices
	// rowIndices contains ACTUAL FILE ROW INDICES (not display indices)
	type annotationToDelete struct {
		annotationID string
		rowIndex     int
	}

	compositeKey := makeCompositeKey(fileHash, opts)
	var annotationsToDelete []annotationToDelete

	rws.workspaceMu.RLock()
	fileAnnots := rws.annotationsMap[compositeKey]
	rws.workspaceMu.RUnlock()

	for i, actualRowIndex := range rowIndices {
		if i < 0 || i >= len(rows) {
			continue // Skip if index is out of bounds for provided rows
		}

		rws.app.Log("debug", fmt.Sprintf("Row %d has actual file row index %d", i, actualRowIndex))

		// Look up annotation by actual file row index (O(1))
		if fileAnnots == nil {
			rws.app.Log("warning", fmt.Sprintf("No annotations found for file %s", compositeKey))
			continue
		}

		annotation := fileAnnots[actualRowIndex]
		if annotation == nil {
			rws.app.Log("warning", fmt.Sprintf("No annotation found for actual row index %d", actualRowIndex))
			continue
		}

		annotationsToDelete = append(annotationsToDelete, annotationToDelete{
			annotationID: annotation.AnnotationID,
			rowIndex:     actualRowIndex,
		})
	}

	if len(annotationsToDelete) == 0 {
		return errors.New("no valid annotations found to delete")
	}

	// Process deletions in batches of 256
	var deletedIDs []string
	const maxBatchSize = 256
	totalAnnotations := len(annotationsToDelete)

	if totalAnnotations == 1 {
		// Single annotation - use individual delete
		annotation := annotationsToDelete[0]
		deleteErr := rws.syncClient.DeleteAnnotation(workspaceID, annotation.annotationID)
		if deleteErr != nil {
			rws.app.Log("error", fmt.Sprintf("Failed to delete annotation %s for row %d: %v", annotation.annotationID, annotation.rowIndex, deleteErr))
		} else {
			deletedIDs = []string{annotation.annotationID}
		}
	} else {
		// Multiple annotations - process in batches
		rws.app.Log("info", fmt.Sprintf("Processing %d annotation deletions in batches of %d", totalAnnotations, maxBatchSize))

		for i := 0; i < totalAnnotations; i += maxBatchSize {
			endIdx := i + maxBatchSize
			if endIdx > totalAnnotations {
				endIdx = totalAnnotations
			}

			batchAnnotations := annotationsToDelete[i:endIdx]

			// Extract annotation IDs for this batch
			batchAnnotationIDs := make([]string, len(batchAnnotations))
			for j, annotation := range batchAnnotations {
				batchAnnotationIDs[j] = annotation.annotationID
			}

			// Try batch delete first
			batchDeletedIDs, deleteErr := rws.syncClient.DeleteAnnotationBatch(workspaceID, batchAnnotationIDs)
			if deleteErr != nil {
				// Fall back to individual deletions for this batch
				for _, annotation := range batchAnnotations {
					fallbackErr := rws.syncClient.DeleteAnnotation(workspaceID, annotation.annotationID)
					if fallbackErr == nil {
						deletedIDs = append(deletedIDs, annotation.annotationID)
					}
				}
			} else {
				deletedIDs = append(deletedIDs, batchDeletedIDs...)
			}
		}
	}

	if len(deletedIDs) == 0 {
		return errors.New("no annotations could be deleted")
	}

	// Update local cache for successfully deleted annotations
	deleteCount := 0
	deletedIDsMap := make(map[string]bool)
	for _, id := range deletedIDs {
		deletedIDsMap[id] = true
	}

	rws.workspaceMu.Lock()
	for _, annotation := range annotationsToDelete {
		if !deletedIDsMap[annotation.annotationID] {
			continue
		}

		// Remove from local cache using row index (O(1) deletion)
		if rws.annotationsMap[compositeKey] != nil {
			delete(rws.annotationsMap[compositeKey], annotation.rowIndex)
		}

		// Remove annotation ID mapping
		delete(rws.annotationIDs, annotation.annotationID)
		deleteCount++
	}
	rws.workspaceMu.Unlock()

	if deleteCount == 0 {
		return errors.New("no annotations were successfully deleted")
	}

	// Invalidate annotation caches
	rws.invalidateAnnotationCaches()

	// Emit workspace updated event
	if rws.ctx != nil {
		runtime.EventsEmit(rws.ctx, "workspace:updated")
	}

	rws.app.Log("info", fmt.Sprintf("Annotation deleted for %d row(s) in remote workspace using pre-fetched data", deleteCount))
	return nil
}

// HasAnnotationsForFile returns true if any annotations exist for the file
// This is used for early-exit optimization in the annotated query
func (rws *RemoteWorkspaceService) HasAnnotationsForFile(fileHash string, opts interfaces.FileOptions) bool {
	rws.workspaceMu.RLock()
	defer rws.workspaceMu.RUnlock()

	compositeKey := makeCompositeKey(fileHash, opts)
	fileAnnots, ok := rws.annotationsMap[compositeKey]
	if !ok {
		return false
	}

	// Simply check if there are any annotations (map length > 0)
	return len(fileAnnots) > 0
}

// IsRowAnnotatedBatch checks multiple rows for annotations using row indices
// This is a highly optimized O(1) lookup per row using the RowIndex field
func (rws *RemoteWorkspaceService) IsRowAnnotatedBatch(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) []bool {
	results := make([]bool, len(rows))
	_ = hashKey // hashKey is no longer needed for row-index-based lookups

	if len(rows) == 0 {
		return results
	}

	rws.workspaceMu.RLock()
	compositeKey := makeCompositeKey(fileHash, opts)
	fileAnnots, ok := rws.annotationsMap[compositeKey]
	rws.workspaceMu.RUnlock()

	if !ok || len(fileAnnots) == 0 {
		return results // All false
	}

	// Simple O(1) lookup per row using RowIndex - no parallelization needed
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
func (rws *RemoteWorkspaceService) IsRowAnnotatedBatchWithColors(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) ([]bool, []string) {
	results := make([]bool, len(rows))
	colors := make([]string, len(rows))
	_ = hashKey // hashKey is no longer needed for row-index-based lookups

	if len(rows) == 0 {
		return results, colors
	}

	rws.workspaceMu.RLock()
	compositeKey := makeCompositeKey(fileHash, opts)
	fileAnnots, ok := rws.annotationsMap[compositeKey]

	// Debug: dump all keys in annotationsMap for this file hash
	var availableKeys []string
	for k := range rws.annotationsMap {
		if strings.HasPrefix(k, fileHash) {
			availableKeys = append(availableKeys, k)
		}
	}
	rws.workspaceMu.RUnlock()

	rws.app.Log("info", fmt.Sprintf("[ANNOTATION_LOOKUP] IsRowAnnotatedBatchWithColors: fileHash=%s, opts=%+v, compositeKey=%s, found=%t, annotCount=%d, availableKeys=%v",
		fileHash, opts, compositeKey, ok, len(fileAnnots), availableKeys))

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

// IsWorkspaceOpen returns true if a remote workspace is currently open
func (rws *RemoteWorkspaceService) IsWorkspaceOpen() bool {
	rws.workspaceMu.RLock()
	defer rws.workspaceMu.RUnlock()
	return rws.workspaceID != ""
}

// GetWorkspaceIdentifier returns the ID of the currently open remote workspace
func (rws *RemoteWorkspaceService) GetWorkspaceIdentifier() string {
	rws.workspaceMu.RLock()
	defer rws.workspaceMu.RUnlock()
	return rws.workspaceID
}

// GetWorkspaceName returns the name of the currently open remote workspace
func (rws *RemoteWorkspaceService) GetWorkspaceName() string {
	rws.workspaceMu.RLock()
	defer rws.workspaceMu.RUnlock()
	return rws.workspaceName
}

// IsRemoteWorkspace returns true for remote workspaces
func (rws *RemoteWorkspaceService) IsRemoteWorkspace() bool {
	return true
}

// GetWorkspaceFiles returns a list of all files tracked in the remote workspace
func (rws *RemoteWorkspaceService) GetWorkspaceFiles() ([]*interfaces.WorkspaceFile, error) {
	rws.workspaceMu.RLock()
	workspaceID := rws.workspaceID
	fileData := rws.fileData
	annotationsMap := rws.annotationsMap
	rws.workspaceMu.RUnlock()

	if workspaceID == "" {
		return nil, errors.New("no remote workspace is open")
	}

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
		// Sort by Options.Key() for consistent ordering (includes jpath, noHeaderRow, timezone, directory options)
		return files[i].Options.Key() < files[j].Options.Key()
	})

	return files, nil
}

// GetWorkspaceFile returns information about a specific file in the workspace
func (rws *RemoteWorkspaceService) GetWorkspaceFile(fileHash string, opts interfaces.FileOptions) (*interfaces.WorkspaceFile, error) {
	rws.workspaceMu.RLock()
	defer rws.workspaceMu.RUnlock()

	file := rws.fileData[makeCompositeKey(fileHash, opts)]
	if file == nil {
		return nil, errors.New("file not in workspace")
	}
	return file, nil
}

// AddFileToWorkspace adds a file to the remote workspace
func (rws *RemoteWorkspaceService) AddFileToWorkspace(fileHash string, opts interfaces.FileOptions, filePath string, description string) error {
	if !rws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	rws.workspaceMu.RLock()
	workspaceID := rws.workspaceID
	rws.workspaceMu.RUnlock()

	if workspaceID == "" {
		return errors.New("no remote workspace is open")
	}

	if !rws.syncClient.IsLoggedIn() {
		return errors.New("not logged in to sync service")
	}

	// Create the file in the remote workspace via sync API
	err := rws.syncClient.CreateFile(workspaceID, fileHash, opts, description, filePath)
	if err != nil {
		return fmt.Errorf("failed to create file in remote workspace: %w", err)
	}

	// Add the file to our local cache
	rws.workspaceMu.Lock()
	compositeKey := makeCompositeKey(fileHash, opts)
	rws.fileData[compositeKey] = &interfaces.WorkspaceFile{
		FilePath:     filePath,
		RelativePath: filePath, // Also set RelativePath for UI to recognize file as "located"
		FileHash:     fileHash,
		Options:      opts,
		Description:  description,
		Annotations:  make([]interfaces.RowAnnotation, 0),
	}
	rws.workspaceMu.Unlock()

	// Emit workspace updated event to notify frontend
	if rws.ctx != nil {
		runtime.EventsEmit(rws.ctx, "workspace:updated")
	}

	rws.app.Log("info", fmt.Sprintf("File added to remote workspace: %s", filePath))
	return nil
}

// UpdateFileDescription updates the description for a file in the remote workspace
func (rws *RemoteWorkspaceService) UpdateFileDescription(fileHash string, opts interfaces.FileOptions, description string) error {
	if !rws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	rws.workspaceMu.RLock()
	workspaceID := rws.workspaceID
	rws.workspaceMu.RUnlock()

	if workspaceID == "" {
		return errors.New("no remote workspace is open")
	}

	if !rws.syncClient.IsLoggedIn() {
		return errors.New("not logged in to sync service")
	}

	// Check if file exists in workspace
	compositeKey := makeCompositeKey(fileHash, opts)
	rws.workspaceMu.RLock()
	file, exists := rws.fileData[compositeKey]
	rws.workspaceMu.RUnlock()

	if !exists {
		return errors.New("file not found in workspace")
	}

	// Update file description via sync API
	err := rws.syncClient.UpdateFileDescription(workspaceID, fileHash, opts, description)
	if err != nil {
		return fmt.Errorf("failed to update file description in remote workspace: %w", err)
	}

	// Update local cache
	rws.workspaceMu.Lock()
	file.Description = description
	rws.fileData[compositeKey] = file
	rws.workspaceMu.Unlock()

	// Emit workspace updated event to notify frontend
	if rws.ctx != nil {
		runtime.EventsEmit(rws.ctx, "workspace:updated")
	}

	rws.app.Log("info", fmt.Sprintf("Updated file description in remote workspace: %s (opts: %+v) -> %s", fileHash, opts, description))
	return nil
}

// RemoveFileFromWorkspace removes a file from the remote workspace
func (rws *RemoteWorkspaceService) RemoveFileFromWorkspace(fileHash string, opts interfaces.FileOptions) error {
	rws.app.Log("debug", fmt.Sprintf("[DELETE_FILE] Starting deletion: fileHash=%s, opts=%+v",
		fileHash, opts))

	if !rws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	rws.workspaceMu.RLock()
	workspaceID := rws.workspaceID
	rws.workspaceMu.RUnlock()

	if workspaceID == "" {
		return errors.New("no remote workspace is open")
	}

	if !rws.syncClient.IsLoggedIn() {
		return errors.New("not logged in to sync service")
	}

	// Check if file exists in workspace
	compositeKey := makeCompositeKey(fileHash, opts)
	rws.app.Log("debug", fmt.Sprintf("[DELETE_FILE] Looking for compositeKey: %s", compositeKey))

	rws.workspaceMu.RLock()
	_, exists := rws.fileData[compositeKey]
	// Log all keys in fileData for debugging
	var allKeys []string
	for key := range rws.fileData {
		allKeys = append(allKeys, key)
	}
	rws.workspaceMu.RUnlock()

	rws.app.Log("debug", fmt.Sprintf("[DELETE_FILE] File exists: %v, All keys in fileData: %v", exists, allKeys))

	if !exists {
		return fmt.Errorf("file not found in workspace (key: %s)", compositeKey)
	}

	// Delete file via sync API - pass full file identifier to delete only this specific variant
	err := rws.syncClient.DeleteFile(workspaceID, fileHash, opts)
	if err != nil {
		return fmt.Errorf("failed to delete file from remote workspace: %w", err)
	}

	// Remove from local cache
	rws.workspaceMu.Lock()
	delete(rws.fileData, compositeKey)
	delete(rws.annotationsMap, compositeKey)
	rws.workspaceMu.Unlock()

	// Emit workspace updated event to notify frontend
	if rws.ctx != nil {
		runtime.EventsEmit(rws.ctx, "workspace:updated")
	}

	rws.app.Log("info", fmt.Sprintf("Removed file from remote workspace: %s (opts: %+v)", fileHash, opts))
	return nil
}

// GetHashKey returns the workspace's hash key for file hashing
func (rws *RemoteWorkspaceService) GetHashKey() []byte {
	rws.workspaceMu.RLock()
	defer rws.workspaceMu.RUnlock()

	// For remote workspaces, the hash key should come from the sync API
	// If we don't have one, it means the workspace wasn't properly loaded
	if rws.hashKey == nil {
		rws.app.Log("warning", "Remote workspace hash key not available - workspace may not be properly loaded")
		return nil
	}

	return rws.hashKey
}

// ExportWorkspaceTimeline exports all annotated rows from all workspace files to a single CSV
func (rws *RemoteWorkspaceService) ExportWorkspaceTimeline() error {
	if !rws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	rws.workspaceMu.RLock()
	hasWorkspace := rws.workspaceID != ""
	rws.workspaceMu.RUnlock()

	if !hasWorkspace {
		return errors.New("no workspace is open")
	}

	// Open save file dialog
	outputPath, err := runtime.SaveFileDialog(rws.ctx, runtime.SaveDialogOptions{
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

	rws.app.Log("info", "Exporting workspace timeline...")

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
	files, err := rws.GetWorkspaceFiles()
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
		rws.app.Log("info", fmt.Sprintf("Processing file: %s", fileInfo.FilePath))

		// For remote workspaces, we need to check if the file exists locally
		// If not, we'll skip this file with a warning
		if fileInfo.FilePath == "" {
			rws.app.Log("warning", fmt.Sprintf("File %s has no local path, skipping timeline export", fileInfo.FileHash))
			continue
		}

		// Check if file exists locally
		if _, err := os.Stat(fileInfo.FilePath); os.IsNotExist(err) {
			rws.app.Log("warning", fmt.Sprintf("File %s not found locally, skipping timeline export", fileInfo.FilePath))
			continue
		}

		// Read the file data
		var header []string
		var rows [][]string

		// Detect file type and read accordingly
		fileType := fileloader.DetectFileType(fileInfo.FilePath)
		if fileType == fileloader.FileTypeJSON {
			if fileInfo.Options.JPath == "" {
				rws.app.Log("error", fmt.Sprintf("JSON file %s requires a JSONPath expression", fileInfo.FilePath))
				continue
			}
			header, err = rws.app.ReadJSONHeader(fileInfo.FilePath, fileInfo.Options.JPath)
			if err != nil {
				rws.app.Log("error", fmt.Sprintf("Failed to read header from %s: %v", fileInfo.FilePath, err))
				continue
			}
			// Read all rows using JSON reader
			reader, _, err := rws.app.GetJSONReader(fileInfo.FilePath, fileInfo.Options.JPath)
			if err != nil {
				rws.app.Log("error", fmt.Sprintf("Failed to read file %s: %v", fileInfo.FilePath, err))
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
			header, err = rws.app.ReadHeader(fileInfo.FilePath)
			if err != nil {
				rws.app.Log("error", fmt.Sprintf("Failed to read header from %s: %v", fileInfo.FilePath, err))
				continue
			}
			// Read all rows
			reader, file, err := rws.app.GetReader(fileInfo.FilePath)
			if err != nil {
				rws.app.Log("error", fmt.Sprintf("Failed to read file %s: %v", fileInfo.FilePath, err))
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
			rws.app.Log("error", fmt.Sprintf("No timestamp column found in file %s", fileInfo.FilePath))
			continue
		}

		// Check each row to see if it's annotated
		// Use row index for annotation lookup (row index = position in file, 0-indexed)
		for rowIndex, row := range rows {
			if len(row) == 0 {
				continue
			}

			isAnnotated, annotation := rws.IsRowAnnotatedByIndex(fileInfo.FileHash, fileInfo.Options, rowIndex)
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

	rws.app.Log("info", fmt.Sprintf("Exported %d annotated rows to %s", len(allAnnotatedRows), filepath.Base(outputPath)))
	return nil
}

// updateWindowTitle updates the window title based on workspace state
func (rws *RemoteWorkspaceService) updateWindowTitle() {
	if rws.ctx == nil {
		return
	}

	rws.workspaceMu.RLock()
	workspaceName := rws.workspaceName
	rws.workspaceMu.RUnlock()

	var title string
	if workspaceName == "" {
		title = "BreachLine"
	} else {
		title = fmt.Sprintf("BreachLine: %s", workspaceName)
	}

	runtime.WindowSetTitle(rws.ctx, title)
}

// startPeriodicSync starts the background sync process
func (rws *RemoteWorkspaceService) startPeriodicSync() {
	rws.syncMu.Lock()
	defer rws.syncMu.Unlock()

	// Stop any existing sync
	if rws.syncTicker != nil {
		rws.syncTicker.Stop()
	}

	// Create new ticker and start sync goroutine
	rws.syncTicker = time.NewTicker(rws.syncInterval)
	rws.syncStop = make(chan bool, 1)

	go rws.periodicSyncWorker()
	rws.app.Log("info", fmt.Sprintf("Started periodic sync with %v interval", rws.syncInterval))
}

// stopPeriodicSync stops the background sync process
func (rws *RemoteWorkspaceService) stopPeriodicSync() {
	rws.syncMu.Lock()
	defer rws.syncMu.Unlock()

	if rws.syncTicker != nil {
		rws.syncTicker.Stop()
		rws.syncTicker = nil
	}

	// Signal stop to worker goroutine
	select {
	case rws.syncStop <- true:
	default:
	}

	rws.app.Log("info", "Stopped periodic sync")
}

// periodicSyncWorker runs the periodic sync in a background goroutine
func (rws *RemoteWorkspaceService) periodicSyncWorker() {
	currentInterval := rws.syncInterval
	maxInterval := 5 * time.Minute // Maximum 5-minute interval

	for {
		select {
		case <-rws.syncStop:
			return
		case <-rws.syncTicker.C:
			// Perform sync
			err := rws.performSync()
			if err != nil {
				// Exponential backoff on failure
				currentInterval = currentInterval * 2
				if currentInterval > maxInterval {
					currentInterval = maxInterval
				}
				rws.app.Log("warning", fmt.Sprintf("Sync failed, increasing interval to %v: %v", currentInterval, err))

				// Update ticker with new interval
				rws.syncMu.Lock()
				if rws.syncTicker != nil {
					rws.syncTicker.Stop()
					rws.syncTicker = time.NewTicker(currentInterval)
				}
				rws.syncMu.Unlock()
			} else {
				// Reset to normal interval on success
				if currentInterval != rws.syncInterval {
					currentInterval = rws.syncInterval
					rws.app.Log("info", fmt.Sprintf("Sync successful, reset interval to %v", currentInterval))

					// Update ticker with normal interval
					rws.syncMu.Lock()
					if rws.syncTicker != nil {
						rws.syncTicker.Stop()
						rws.syncTicker = time.NewTicker(currentInterval)
					}
					rws.syncMu.Unlock()
				}
			}
		}
	}
}

// performSync performs a full sync of workspace data from the server
func (rws *RemoteWorkspaceService) performSync() error {
	// Prevent concurrent sync operations
	rws.syncMu.Lock()
	if rws.isSync {
		rws.syncMu.Unlock()
		return nil // Skip if already syncing
	}
	rws.isSync = true
	rws.syncMu.Unlock()

	defer func() {
		rws.syncMu.Lock()
		rws.isSync = false
		rws.syncMu.Unlock()
	}()

	// Check if we still have a workspace open
	rws.workspaceMu.RLock()
	workspaceID := rws.workspaceID
	rws.workspaceMu.RUnlock()

	if workspaceID == "" {
		return errors.New("no workspace open")
	}

	if !rws.syncClient.IsLoggedIn() {
		return errors.New("not logged in to sync service")
	}

	// Get latest workspace details to check UpdatedAt
	detailsRaw, err := rws.syncClient.GetWorkspaceDetailsForClient(workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get workspace details during sync: %w", err)
	}

	// Convert to get UpdatedAt
	var details RemoteWorkspaceDetails
	detailsData, _ := json.Marshal(detailsRaw)
	json.Unmarshal(detailsData, &details)

	// Check if UpdatedAt has changed
	rws.workspaceMu.RLock()
	lastKnownUpdatedAt := rws.lastKnownUpdatedAt
	rws.workspaceMu.RUnlock()

	updatedAtChanged := details.UpdatedAt != lastKnownUpdatedAt

	if updatedAtChanged {
		rws.app.Log("debug", fmt.Sprintf("Workspace UpdatedAt changed from %s to %s - invalidating annotation caches",
			lastKnownUpdatedAt, details.UpdatedAt))
		rws.invalidateAnnotationCaches()
	}

	// Get latest workspace data from server
	annotationsRaw, err := rws.syncClient.GetWorkspaceAnnotationsForClient(workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get workspace annotations: %w", err)
	}

	filesRaw, err := rws.syncClient.GetWorkspaceFilesForClient(workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get workspace files: %w", err)
	}

	// Convert server data
	var serverAnnotations []RemoteAnnotation
	data, _ := json.Marshal(annotationsRaw)
	json.Unmarshal(data, &serverAnnotations)

	var serverFiles []map[string]interface{}
	data, _ = json.Marshal(filesRaw)
	json.Unmarshal(data, &serverFiles)

	// Apply server changes (server wins conflict resolution)
	err = rws.applyServerChanges(serverAnnotations, serverFiles)
	if err != nil {
		return fmt.Errorf("failed to apply server changes: %w", err)
	}

	// Update stored UpdatedAt timestamp after successful sync
	if updatedAtChanged {
		rws.workspaceMu.Lock()
		rws.lastKnownUpdatedAt = details.UpdatedAt
		rws.workspaceMu.Unlock()
	}

	// Emit workspace updated event to refresh UI
	if rws.ctx != nil {
		runtime.EventsEmit(rws.ctx, "workspace:updated")
	}

	return nil
}

// applyServerChanges applies server data to local cache (server wins)
func (rws *RemoteWorkspaceService) applyServerChanges(serverAnnotations []RemoteAnnotation, serverFiles []map[string]interface{}) error {
	rws.workspaceMu.Lock()
	defer rws.workspaceMu.Unlock()

	// Clear existing data and rebuild from server
	rws.annotationsMap = make(map[string]map[int]*interfaces.RowAnnotation)
	rws.annotationIDs = make(map[string]string)

	// Rebuild file data from server files
	fileMap := make(map[string]*interfaces.WorkspaceFile)

	// First, create file entries from server files
	for _, fileData := range serverFiles {
		fileHash, _ := fileData["file_hash"].(string)
		description, _ := fileData["description"].(string)

		// Extract options from the nested "options" object (as returned by sync-api)
		opts := extractFileOptionsFromMap(fileData)

		// // Debug: log the extracted options during sync
		// rws.app.Log("debug", fmt.Sprintf("[SYNC_UPDATE] File %s: options=%+v", fileHash, opts))

		compositeKey := makeCompositeKey(fileHash, opts)

		// Preserve existing file path and relative path if available
		var filePath string
		var relativePath string
		if existingFile, exists := rws.fileData[compositeKey]; exists {
			filePath = existingFile.FilePath
			relativePath = existingFile.RelativePath
		}

		fileMap[compositeKey] = &interfaces.WorkspaceFile{
			FilePath:     filePath,
			RelativePath: relativePath,
			FileHash:     fileHash,
			Options:      opts,
			Description:  description,
			Annotations:  make([]interfaces.RowAnnotation, 0),
		}
	}

	// Then, process server annotations
	// Only attach annotations to files that have actual file records - don't create ghost files
	for _, annot := range serverAnnotations {
		// Use the annotation's options to construct the composite key
		// This ensures annotations are correctly associated with files based on their settings
		annotOpts := interfaces.FileOptions{
			JPath:                  annot.Options.JPath,
			NoHeaderRow:            annot.Options.NoHeaderRow,
			IngestTimezoneOverride: annot.Options.IngestTimezoneOverride,
			// Directory loading options
			IsDirectory:         annot.Options.IsDirectory,
			FilePattern:         annot.Options.FilePattern,
			IncludeSourceColumn: annot.Options.IncludeSourceColumn,
		}
		compositeKey := makeCompositeKey(annot.FileHash, annotOpts)

		// Only attach annotations to existing file records - don't create ghost files
		if _, exists := fileMap[compositeKey]; !exists {
			rws.app.Log("debug", fmt.Sprintf("[SYNC_UPDATE] Skipping orphaned annotation %s - no file record exists for compositeKey=%s",
				annot.AnnotationID, compositeKey))
			continue
		}

		// Convert server annotation to local format using the actual RowIndex from the API
		rowAnnotation := interfaces.RowAnnotation{
			AnnotationID: annot.AnnotationID,
			RowIndex:     annot.RowIndex,
			Note:         annot.Note,
			Color:        annot.Color,
		}

		fileMap[compositeKey].Annotations = append(fileMap[compositeKey].Annotations, rowAnnotation)

		// Build annotations map for fast lookup
		if rws.annotationsMap[compositeKey] == nil {
			rws.annotationsMap[compositeKey] = make(map[int]*interfaces.RowAnnotation)
		}

		annotCopy := rowAnnotation
		rws.annotationsMap[compositeKey][annot.RowIndex] = &annotCopy

		// Store annotation ID for deletion lookups
		rws.annotationIDs[annot.AnnotationID] = compositeKey
	}

	// Update file data
	rws.fileData = fileMap

	return nil
}

// NOTE: generateColumnHashSignature has been removed.
// Annotations are now identified by RowIndex instead of column hash signatures.

// RefreshFileLocations reloads file locations from the sync API and updates file paths
func (rws *RemoteWorkspaceService) RefreshFileLocations() error {
	rws.workspaceMu.Lock()
	defer rws.workspaceMu.Unlock()

	workspaceIdentifier := rws.workspaceID
	if workspaceIdentifier == "" {
		return fmt.Errorf("no remote workspace is open")
	}

	// Get file locations for this instance and apply them to files
	fileLocationsRaw, err := rws.syncClient.GetFileLocationsForInstance()
	if err != nil {
		return fmt.Errorf("failed to get file locations: %w", err)
	}

	// Convert interface{} to ListFileLocationsResponse first
	var response map[string]interface{}
	if locationsData, err := json.Marshal(fileLocationsRaw); err == nil {
		if err := json.Unmarshal(locationsData, &response); err == nil {
			// Extract the file_locations array from the response
			if fileLocationsArray, ok := response["file_locations"].([]interface{}); ok {
				rws.app.Log("info", fmt.Sprintf("Refreshed %d file locations from API", len(fileLocationsArray)))

				// Apply file locations to matching files
				for _, locationInterface := range fileLocationsArray {
					if location, ok := locationInterface.(map[string]interface{}); ok {
						fileHash, _ := location["file_hash"].(string)
						filePath, _ := location["file_path"].(string)
						workspaceID, _ := location["workspace_id"].(string)

						rws.app.Log("debug", fmt.Sprintf("Processing file location: hash=%s, path=%s, workspace=%s", fileHash, filePath, workspaceID))

						// Only apply locations for files in the current workspace
						if workspaceID == workspaceIdentifier && fileHash != "" && filePath != "" {
							// Find all files with this hash (there might be multiple jpaths)
							for key, file := range rws.fileData {
								if file.FileHash == fileHash {
									rws.app.Log("debug", fmt.Sprintf("Updating file path for key %s: %s", key, filePath))
									// Set the relative path to the local file location
									file.RelativePath = filePath
									// If FilePath is empty, also set it to have a fallback
									if file.FilePath == "" {
										file.FilePath = filePath
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// RegisterCacheInvalidator registers a cache invalidator for annotation-dependent caches
func (rws *RemoteWorkspaceService) RegisterCacheInvalidator(invalidator CacheInvalidator) {
	rws.invalidatorMu.Lock()
	defer rws.invalidatorMu.Unlock()
	rws.cacheInvalidators = append(rws.cacheInvalidators, invalidator)
}

// invalidateAnnotationCaches triggers invalidation of all registered annotation-dependent caches
func (rws *RemoteWorkspaceService) invalidateAnnotationCaches() {
	rws.invalidatorMu.RLock()
	invalidators := make([]CacheInvalidator, len(rws.cacheInvalidators))
	copy(invalidators, rws.cacheInvalidators)
	rws.invalidatorMu.RUnlock()

	rws.app.Log("debug", fmt.Sprintf("[CACHE_INVALIDATE_TRIGGER] Remote workspace %s triggering invalidation for %d registered invalidators", rws.workspaceID, len(invalidators)))

	for _, invalidator := range invalidators {
		invalidator.InvalidateAnnotationCaches(rws.workspaceID)
	}

	rws.app.Log("debug", fmt.Sprintf("Invalidated annotation-dependent caches for remote workspace %s", rws.workspaceID))
}

// DeleteAnnotationByID deletes an annotation by its ID
func (rws *RemoteWorkspaceService) DeleteAnnotationByID(annotationID string) error {
	if !rws.app.IsLicensed() {
		return errors.New("this feature requires a valid license")
	}

	rws.workspaceMu.RLock()
	workspaceID := rws.workspaceID
	rws.workspaceMu.RUnlock()

	if workspaceID == "" {
		return errors.New("no remote workspace is open")
	}

	if !rws.syncClient.IsLoggedIn() {
		return errors.New("not logged in to sync service")
	}

	// Delete via sync API
	err := rws.syncClient.DeleteAnnotation(workspaceID, annotationID)
	if err != nil {
		return fmt.Errorf("failed to delete annotation: %w", err)
	}

	// Remove from local cache
	rws.workspaceMu.Lock()

	// Find the annotation's composite key from annotationIDs
	compositeKey, exists := rws.annotationIDs[annotationID]
	if exists {
		// Find and remove the annotation by searching for its ID
		if fileAnnots, ok := rws.annotationsMap[compositeKey]; ok {
			for rowIndex, annot := range fileAnnots {
				if annot != nil && annot.AnnotationID == annotationID {
					delete(rws.annotationsMap[compositeKey], rowIndex)
					break
				}
			}
		}
	}

	// Remove from annotationIDs map
	delete(rws.annotationIDs, annotationID)
	rws.workspaceMu.Unlock()

	// Invalidate caches
	rws.invalidateAnnotationCaches()

	// Emit workspace updated event
	if rws.ctx != nil {
		runtime.EventsEmit(rws.ctx, "workspace:updated")
	}

	rws.app.Log("info", fmt.Sprintf("Deleted annotation: %s", annotationID))
	return nil
}

// GetAnnotationByID retrieves an annotation by its ID
func (rws *RemoteWorkspaceService) GetAnnotationByID(annotationID string) (*interfaces.AnnotationResult, error) {
	rws.workspaceMu.RLock()
	defer rws.workspaceMu.RUnlock()

	if rws.workspaceID == "" {
		return nil, errors.New("no remote workspace is open")
	}

	// Find the annotation's composite key from annotationIDs
	compositeKey, exists := rws.annotationIDs[annotationID]
	if !exists {
		return nil, fmt.Errorf("annotation not found: %s", annotationID)
	}

	// Search for the annotation in the file's annotations
	if fileAnnots, ok := rws.annotationsMap[compositeKey]; ok {
		for _, annot := range fileAnnots {
			if annot != nil && annot.AnnotationID == annotationID {
				return &interfaces.AnnotationResult{
					ID:    annotationID,
					Note:  annot.Note,
					Color: annot.Color,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("annotation not found: %s", annotationID)
}

// IsRowAnnotatedBatchWithInfo returns full annotation info for batch of rows
// This populates Row.Annotation for caching and allows ID-based operations
func (rws *RemoteWorkspaceService) IsRowAnnotatedBatchWithInfo(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) []*interfaces.RowAnnotationInfo {
	results := make([]*interfaces.RowAnnotationInfo, len(rows))
	_ = hashKey // hashKey is no longer needed for row-index-based lookups

	if len(rows) == 0 {
		return results
	}

	rws.workspaceMu.RLock()
	compositeKey := makeCompositeKey(fileHash, opts)
	fileAnnots, ok := rws.annotationsMap[compositeKey]
	rws.workspaceMu.RUnlock()

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
func (rws *RemoteWorkspaceService) GetFileAnnotations(fileHash string, opts interfaces.FileOptions) ([]*interfaces.FileAnnotationInfo, error) {
	rws.workspaceMu.RLock()
	defer rws.workspaceMu.RUnlock()

	compositeKey := makeCompositeKey(fileHash, opts)
	fileAnnots, ok := rws.annotationsMap[compositeKey]
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
