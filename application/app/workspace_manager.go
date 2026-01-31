package app

import (
	"context"
	"fmt"
	"time"

	"breachline/app/interfaces"
	"breachline/app/workspace"
)

// WorkspaceManager manages workspace services and provides a unified interface
type WorkspaceManager struct {
	app            interfaces.AppService
	ctx            context.Context
	currentService workspace.WorkspaceService
	localService   *workspace.LocalWorkspaceService
	remoteService  *workspace.RemoteWorkspaceService
	syncClient     workspace.SyncClient
}

// NewWorkspaceManager creates a new workspace manager
func NewWorkspaceManager() *WorkspaceManager {
	return &WorkspaceManager{
		localService: workspace.NewLocalWorkspaceService(),
	}
}

// SetSyncClient sets the sync client for remote workspace operations
func (wm *WorkspaceManager) SetSyncClient(syncClient workspace.SyncClient) {
	wm.syncClient = syncClient
	wm.remoteService = workspace.NewRemoteWorkspaceService(syncClient)
	if wm.app != nil {
		wm.remoteService.SetApp(wm.app)
	}
	if wm.ctx != nil {
		wm.remoteService.Startup(wm.ctx)
	}
}

// SetApp sets the app reference for all workspace services
func (wm *WorkspaceManager) SetApp(app interfaces.AppService) {
	wm.app = app
	wm.localService.SetApp(app)
	if wm.remoteService != nil {
		wm.remoteService.SetApp(app)
	}
}

// Startup initializes the workspace manager
func (wm *WorkspaceManager) Startup(ctx context.Context) {
	wm.ctx = ctx
	wm.localService.Startup(ctx)
	if wm.remoteService != nil {
		wm.remoteService.Startup(ctx)
	}

	// Default to local service initially
	wm.currentService = wm.localService
}

// GetCurrentService returns the currently active workspace service
func (wm *WorkspaceManager) GetCurrentService() workspace.WorkspaceService {
	if wm.currentService == nil {
		return wm.localService // fallback to local service
	}
	return wm.currentService
}

// CreateWorkspace creates a new workspace (defaults to local)
func (wm *WorkspaceManager) CreateWorkspace(workspaceIdentifier string) error {
	wm.currentService = wm.localService
	return wm.localService.CreateWorkspace(workspaceIdentifier)
}

// CreateLocalWorkspace creates a new local workspace
func (wm *WorkspaceManager) CreateLocalWorkspace(workspaceIdentifier string) error {
	wm.currentService = wm.localService
	return wm.localService.CreateWorkspace(workspaceIdentifier)
}

// OpenWorkspace opens a workspace (defaults to local)
func (wm *WorkspaceManager) OpenWorkspace(workspaceIdentifier string) error {
	// Close current workspace if one is open
	if wm.IsWorkspaceOpen() {
		if err := wm.CloseWorkspace(); err != nil {
			wm.app.Log("warning", fmt.Sprintf("Failed to close current workspace before opening new one: %v", err))
		}
	}

	wm.currentService = wm.localService
	err := wm.localService.OpenWorkspace(workspaceIdentifier)
	if err != nil {
		return err
	}

	// Register cache invalidator after successful workspace opening
	if wm.app != nil && wm.app.GetQueryCache() != nil {
		if cache, ok := wm.app.GetQueryCache().(workspace.CacheInvalidator); ok {
			wm.currentService.RegisterCacheInvalidator(cache)
			wm.app.Log("debug", "[CACHE_INVALIDATOR_REGISTER] Registered query cache for annotation invalidation")

			// Now invalidate annotation caches to clear stale data (must happen AFTER registration)
			if invalidator, ok := wm.currentService.(interface{ InvalidateAnnotationCaches() }); ok {
				invalidator.InvalidateAnnotationCaches()
				wm.app.Log("debug", "[CACHE_INVALIDATE] Invalidated annotation caches after workspace open")
			}
		} else {
			wm.app.Log("debug", "[CACHE_INVALIDATOR_FAIL] Failed to cast query cache to CacheInvalidator interface")
		}
	} else {
		wm.app.Log("debug", "[CACHE_INVALIDATOR_SKIP] App or query cache is nil, skipping invalidator registration")
	}

	return nil
}

// OpenLocalWorkspace opens a local workspace
func (wm *WorkspaceManager) OpenLocalWorkspace(workspaceIdentifier string) error {
	// Close current workspace if one is open
	if wm.IsWorkspaceOpen() {
		if err := wm.CloseWorkspace(); err != nil {
			wm.app.Log("warning", fmt.Sprintf("Failed to close current workspace before opening new one: %v", err))
		}
	}

	wm.currentService = wm.localService
	err := wm.localService.OpenWorkspace(workspaceIdentifier)
	if err != nil {
		return err
	}

	// Register cache invalidator after successful workspace opening
	if wm.app != nil && wm.app.GetQueryCache() != nil {
		if cache, ok := wm.app.GetQueryCache().(workspace.CacheInvalidator); ok {
			wm.currentService.RegisterCacheInvalidator(cache)
			wm.app.Log("debug", "Registered query cache for annotation invalidation")

			// Now invalidate annotation caches to clear stale data (must happen AFTER registration)
			if invalidator, ok := wm.currentService.(interface{ InvalidateAnnotationCaches() }); ok {
				invalidator.InvalidateAnnotationCaches()
				wm.app.Log("debug", "[CACHE_INVALIDATE] Invalidated annotation caches after workspace open")
			}
		}
	}

	return nil
}

// OpenRemoteWorkspace opens a remote workspace
func (wm *WorkspaceManager) OpenRemoteWorkspace(workspaceIdentifier string) error {
	if wm.remoteService == nil {
		return fmt.Errorf("remote workspace service not initialized - sync client not set")
	}

	// Close current workspace if one is open
	if wm.IsWorkspaceOpen() {
		if err := wm.CloseWorkspace(); err != nil {
			wm.app.Log("warning", fmt.Sprintf("Failed to close current workspace before opening new one: %v", err))
		}
	}

	wm.currentService = wm.remoteService
	err := wm.remoteService.OpenWorkspace(workspaceIdentifier)
	if err != nil {
		return err
	}

	// Register cache invalidator after successful workspace opening
	if wm.app != nil && wm.app.GetQueryCache() != nil {
		if cache, ok := wm.app.GetQueryCache().(workspace.CacheInvalidator); ok {
			wm.currentService.RegisterCacheInvalidator(cache)
			wm.app.Log("debug", "Registered query cache for annotation invalidation")

			// Now invalidate annotation caches to clear stale data (must happen AFTER registration)
			if invalidator, ok := wm.currentService.(interface{ InvalidateAnnotationCaches() }); ok {
				invalidator.InvalidateAnnotationCaches()
				wm.app.Log("debug", "[CACHE_INVALIDATE] Invalidated annotation caches after workspace open")
			}
		}
	}

	return nil
}

// CreateRemoteWorkspace creates a new remote workspace
func (wm *WorkspaceManager) CreateRemoteWorkspace(name string) error {
	if wm.remoteService == nil {
		return fmt.Errorf("remote workspace service not initialized - sync client not set")
	}
	// Note: We don't set currentService here as CreateWorkspace doesn't open the workspace
	return wm.remoteService.CreateWorkspace(name)
}

// CreateAndOpenRemoteWorkspace creates a new remote workspace and opens it immediately
func (wm *WorkspaceManager) CreateAndOpenRemoteWorkspace(name string) error {
	if wm.remoteService == nil {
		return fmt.Errorf("remote workspace service not initialized - sync client not set")
	}

	// Create the workspace and get its ID
	workspaceID, err := wm.remoteService.CreateWorkspaceAndReturnID(name)
	if err != nil {
		return fmt.Errorf("failed to create remote workspace: %w", err)
	}

	// Close current workspace if one is open before opening the new one
	if wm.IsWorkspaceOpen() {
		if err := wm.CloseWorkspace(); err != nil {
			wm.app.Log("warning", fmt.Sprintf("Failed to close current workspace before opening new one: %v", err))
		}
	}

	// Open the newly created workspace using its ID with retry logic
	wm.currentService = wm.remoteService

	// Retry opening the workspace for up to 10 seconds
	const maxRetryDuration = 10 * time.Second
	const retryInterval = 500 * time.Millisecond

	startTime := time.Now()
	var lastErr error

	for time.Since(startTime) < maxRetryDuration {
		err = wm.remoteService.OpenWorkspace(workspaceID)
		if err == nil {
			// Successfully opened the workspace
			// Register cache invalidator after successful workspace opening
			if wm.app != nil && wm.app.GetQueryCache() != nil {
				if cache, ok := wm.app.GetQueryCache().(workspace.CacheInvalidator); ok {
					wm.currentService.RegisterCacheInvalidator(cache)
					wm.app.Log("debug", "Registered query cache for annotation invalidation")

					// Now invalidate annotation caches to clear stale data (must happen AFTER registration)
					if invalidator, ok := wm.currentService.(interface{ InvalidateAnnotationCaches() }); ok {
						invalidator.InvalidateAnnotationCaches()
						wm.app.Log("debug", "[CACHE_INVALIDATE] Invalidated annotation caches after workspace open")
					}
				}
			}
			return nil
		}

		lastErr = err

		// Wait before retrying
		time.Sleep(retryInterval)
	}

	// If we get here, all retries failed
	return fmt.Errorf("failed to open newly created remote workspace after %v: %w", maxRetryDuration, lastErr)
}

// CloseWorkspace closes the current workspace
func (wm *WorkspaceManager) CloseWorkspace() error {
	if wm.currentService != nil {
		err := wm.currentService.CloseWorkspace()
		wm.currentService = wm.localService // reset to local service
		return err
	}
	return nil
}

// Delegate methods to current service
func (wm *WorkspaceManager) ChooseAndOpenWorkspace() error {
	// Close current workspace if one is open
	if wm.IsWorkspaceOpen() {
		if err := wm.CloseWorkspace(); err != nil {
			wm.app.Log("warning", fmt.Sprintf("Failed to close current workspace before opening new one: %v", err))
		}
	}

	// This will open a local workspace for now
	wm.currentService = wm.localService
	err := wm.localService.ChooseAndOpenWorkspace()
	if err != nil {
		return err
	}

	// Register cache invalidator after successful workspace opening
	if wm.app != nil && wm.app.GetQueryCache() != nil {
		if cache, ok := wm.app.GetQueryCache().(workspace.CacheInvalidator); ok {
			wm.currentService.RegisterCacheInvalidator(cache)
			wm.app.Log("debug", "[CACHE_INVALIDATOR_REGISTER] Registered query cache for annotation invalidation (ChooseAndOpen path)")

			// Now invalidate annotation caches to clear stale data (must happen AFTER registration)
			if invalidator, ok := wm.currentService.(interface{ InvalidateAnnotationCaches() }); ok {
				invalidator.InvalidateAnnotationCaches()
				wm.app.Log("debug", "[CACHE_INVALIDATE] Invalidated annotation caches after workspace open")
			}
		} else {
			wm.app.Log("debug", "[CACHE_INVALIDATOR_FAIL] Failed to cast query cache to CacheInvalidator interface (ChooseAndOpen path)")
		}
	}

	return nil
}

func (wm *WorkspaceManager) IsWorkspaceOpen() bool {
	if wm.currentService != nil {
		return wm.currentService.IsWorkspaceOpen()
	}
	return false
}

func (wm *WorkspaceManager) GetWorkspaceIdentifier() string {
	if wm.currentService != nil {
		return wm.currentService.GetWorkspaceIdentifier()
	}
	return ""
}

func (wm *WorkspaceManager) GetWorkspaceName() string {
	if wm.currentService != nil {
		return wm.currentService.GetWorkspaceName()
	}
	return ""
}

func (wm *WorkspaceManager) IsRemoteWorkspace() bool {
	if wm.currentService != nil {
		return wm.currentService.IsRemoteWorkspace()
	}
	return false
}

func (wm *WorkspaceManager) GetWorkspaceFiles() ([]*interfaces.WorkspaceFile, error) {
	if wm.currentService != nil {
		return wm.currentService.GetWorkspaceFiles()
	}
	return nil, fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) GetWorkspaceFile(fileHash string, opts interfaces.FileOptions) (*interfaces.WorkspaceFile, error) {
	if wm.currentService != nil {
		return wm.currentService.GetWorkspaceFile(fileHash, opts)
	}
	return nil, fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) AddFileToWorkspace(fileHash string, opts interfaces.FileOptions, filePath string, description string) error {
	if wm.currentService != nil {
		return wm.currentService.AddFileToWorkspace(fileHash, opts, filePath, description)
	}
	return fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) UpdateFileDescription(fileHash string, opts interfaces.FileOptions, description string) error {
	if wm.currentService != nil {
		return wm.currentService.UpdateFileDescription(fileHash, opts, description)
	}
	return fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) RemoveFileFromWorkspace(fileHash string, opts interfaces.FileOptions) error {
	if wm.currentService != nil {
		return wm.currentService.RemoveFileFromWorkspace(fileHash, opts)
	}
	return fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) AddAnnotations(fileHash string, opts interfaces.FileOptions, rowIndices []int, timeField string, note string, color string, query string) error {
	if wm.currentService != nil {
		return wm.currentService.AddAnnotations(fileHash, opts, rowIndices, timeField, note, color, query)
	}
	return fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) AddAnnotationsWithRows(fileHash string, opts interfaces.FileOptions, rowIndices []int, rows [][]string, header []string, timeField string, note string, color string, query string) error {
	if wm.currentService != nil {
		return wm.currentService.AddAnnotationsWithRows(fileHash, opts, rowIndices, rows, header, timeField, note, color, query)
	}
	return fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) GetRowAnnotation(fileHash string, opts interfaces.FileOptions, rowIndex int, query string, timeField string) (*interfaces.AnnotationResult, error) {
	if wm.currentService != nil {
		return wm.currentService.GetRowAnnotation(fileHash, opts, rowIndex, query, timeField)
	}
	return nil, fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) GetRowAnnotations(fileHash string, opts interfaces.FileOptions, rowIndices []int, query string, timeField string) (map[int]*interfaces.AnnotationResult, error) {
	if wm.currentService != nil {
		return wm.currentService.GetRowAnnotations(fileHash, opts, rowIndices, query, timeField)
	}
	return nil, fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) GetRowAnnotationsWithRows(fileHash string, opts interfaces.FileOptions, rowIndices []int, rows [][]string, header []string) (map[int]*interfaces.AnnotationResult, error) {
	if wm.currentService != nil {
		return wm.currentService.GetRowAnnotationsWithRows(fileHash, opts, rowIndices, rows, header)
	}
	return nil, fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) DeleteRowAnnotations(fileHash string, opts interfaces.FileOptions, rowIndices []int, timeField string, query string) error {
	if wm.currentService != nil {
		return wm.currentService.DeleteRowAnnotations(fileHash, opts, rowIndices, timeField, query)
	}
	return fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) DeleteRowAnnotationsWithRows(fileHash string, opts interfaces.FileOptions, rowIndices []int, rows [][]string, header []string, timeField string, query string) error {
	if wm.currentService != nil {
		return wm.currentService.DeleteRowAnnotationsWithRows(fileHash, opts, rowIndices, rows, header, timeField, query)
	}
	return fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) GetHashKey() []byte {
	if wm.currentService != nil {
		return wm.currentService.GetHashKey()
	}
	return nil
}

func (wm *WorkspaceManager) ExportWorkspaceTimeline() error {
	if wm.currentService != nil {
		return wm.currentService.ExportWorkspaceTimeline()
	}
	return fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) RefreshFileLocations() error {
	if wm.currentService != nil {
		return wm.currentService.RefreshFileLocations()
	}
	return fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) HasAnnotationsForFile(fileHash string, opts interfaces.FileOptions) bool {
	if wm.currentService != nil {
		return wm.currentService.HasAnnotationsForFile(fileHash, opts)
	}
	return false
}

func (wm *WorkspaceManager) IsRowAnnotatedBatch(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) []bool {
	if wm.currentService != nil {
		return wm.currentService.IsRowAnnotatedBatch(fileHash, opts, rows, hashKey)
	}
	return make([]bool, len(rows))
}

func (wm *WorkspaceManager) IsRowAnnotatedBatchWithColors(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) ([]bool, []string) {
	if wm.currentService != nil {
		return wm.currentService.IsRowAnnotatedBatchWithColors(fileHash, opts, rows, hashKey)
	}
	return make([]bool, len(rows)), make([]string, len(rows))
}

func (wm *WorkspaceManager) IsRowAnnotatedBatchWithInfo(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) []*interfaces.RowAnnotationInfo {
	if wm.currentService != nil {
		return wm.currentService.IsRowAnnotatedBatchWithInfo(fileHash, opts, rows, hashKey)
	}
	return make([]*interfaces.RowAnnotationInfo, len(rows))
}

func (wm *WorkspaceManager) DeleteAnnotationByID(annotationID string) error {
	if wm.currentService != nil {
		return wm.currentService.DeleteAnnotationByID(annotationID)
	}
	return fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) GetAnnotationByID(annotationID string) (*interfaces.AnnotationResult, error) {
	if wm.currentService != nil {
		return wm.currentService.GetAnnotationByID(annotationID)
	}
	return nil, fmt.Errorf("no workspace service available")
}

func (wm *WorkspaceManager) GetFileAnnotations(fileHash string, opts interfaces.FileOptions) ([]*interfaces.FileAnnotationInfo, error) {
	if wm.currentService != nil {
		return wm.currentService.GetFileAnnotations(fileHash, opts)
	}
	return nil, fmt.Errorf("no workspace service available")
}
