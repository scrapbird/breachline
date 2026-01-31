package workspace

import (
	"context"
	"crypto/rand"
	"fmt"

	"breachline/app/interfaces"

	"github.com/google/uuid"
)

// CacheInvalidator interface for annotation-dependent caches
type CacheInvalidator interface {
	InvalidateAnnotationCaches(workspaceID string)
}

type WorkspaceService interface {
	SetApp(app interfaces.AppService)
	Startup(ctx context.Context)
	CreateWorkspace(workspaceIdentifer string) error
	ChooseAndOpenWorkspace() error
	OpenWorkspace(workspaceIdentifier string) error
	CloseWorkspace() error
	AddAnnotations(fileHash string, opts interfaces.FileOptions, rowIndicies []int, timeField string, note string, color string, query string) error
	AddAnnotationsWithRows(fileHash string, opts interfaces.FileOptions, rowIndicies []int, rows [][]string, header []string, timeField string, note string, color string, query string) error
	GetRowAnnotation(fileHash string, opts interfaces.FileOptions, rowIndex int, query string, timeField string) (*interfaces.AnnotationResult, error)
	GetRowAnnotations(fileHash string, opts interfaces.FileOptions, rowIndices []int, query string, timeField string) (map[int]*interfaces.AnnotationResult, error)
	GetRowAnnotationsWithRows(fileHash string, opts interfaces.FileOptions, rowIndices []int, rows [][]string, header []string) (map[int]*interfaces.AnnotationResult, error)
	DeleteRowAnnotations(fileHash string, opts interfaces.FileOptions, rowIndicies []int, timeField string, query string) error
	DeleteRowAnnotationsWithRows(fileHash string, opts interfaces.FileOptions, rowIndices []int, rows [][]string, header []string, timeField string, query string) error
	IsWorkspaceOpen() bool
	GetWorkspaceIdentifier() string
	GetWorkspaceName() string
	IsRemoteWorkspace() bool
	GetWorkspaceFiles() ([]*interfaces.WorkspaceFile, error)
	GetWorkspaceFile(fileHash string, opts interfaces.FileOptions) (*interfaces.WorkspaceFile, error)
	AddFileToWorkspace(fileHash string, opts interfaces.FileOptions, filePath string, description string) error
	UpdateFileDescription(fileHash string, opts interfaces.FileOptions, description string) error
	RemoveFileFromWorkspace(fileHash string, opts interfaces.FileOptions) error
	GetHashKey() []byte
	ExportWorkspaceTimeline() error
	RefreshFileLocations() error // For remote workspaces only, no-op for local workspaces

	// Batch annotation checking for performance optimization
	HasAnnotationsForFile(fileHash string, opts interfaces.FileOptions) bool
	IsRowAnnotatedBatch(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) []bool
	IsRowAnnotatedBatchWithColors(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) ([]bool, []string)
	// IsRowAnnotatedBatchWithInfo returns full annotation info for batch of rows
	// This populates Row.Annotation for caching and allows ID-based operations
	IsRowAnnotatedBatchWithInfo(fileHash string, opts interfaces.FileOptions, rows []*interfaces.Row, hashKey []byte) []*interfaces.RowAnnotationInfo

	// Annotation ID-based operations (faster than hash-based when ID is known)
	DeleteAnnotationByID(annotationID string) error
	GetAnnotationByID(annotationID string) (*interfaces.AnnotationResult, error)

	// Get all annotations for a file (for annotation panel)
	GetFileAnnotations(fileHash string, opts interfaces.FileOptions) ([]*interfaces.FileAnnotationInfo, error)

	// Cache invalidation support
	RegisterCacheInvalidator(invalidator CacheInvalidator)
}

// generateHashKey creates a random 32-byte key for HighwayHash
func GenerateHashKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	return key, nil
}

// makeCompositeKey creates a composite key from file hash and options
// This allows the same file to be tracked separately with different options
func makeCompositeKey(fileHash string, opts interfaces.FileOptions) string {
	return fileHash + "::" + opts.Key()
}

// NOTE: The following deprecated column hash functions have been removed:
// - splitCompositeKey (unused after row-index refactor)
// - calculateColumnHash (unused after row-index refactor)
// - calculateRowColumnHashes (unused after row-index refactor)
// - extractHashFromColumnHash (unused after row-index refactor)
// - getFirstColumnHash (unused after row-index refactor)
// Annotations are now identified by RowIndex instead of column hashes.

// GenerateAnnotationID creates a new UUID-based annotation ID
func GenerateAnnotationID() string {
	return fmt.Sprintf("ann_%s", uuid.New().String())
}
