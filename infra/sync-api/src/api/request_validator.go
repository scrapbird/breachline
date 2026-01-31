package api

import (
	"fmt"
	"regexp"
	"strings"
)

// Size limit constants for API data fields
const (
	MaxAnnotationNoteSize  = 512
	MaxAnnotationColorSize = 16
	MaxWorkspaceNameSize   = 64
	MaxFilePathSize        = 1024
	MaxJPathSize           = 512
	MaxIDSize              = 46 // UUID (36) + prefix buffer (10)
	MaxHashSize            = 64 // Allow for base64 or hex encoding
)

// RequestValidator provides validation methods for API requests
type RequestValidator struct {
	errors []string
}

// NewRequestValidator creates a new validator instance
func NewRequestValidator() *RequestValidator {
	return &RequestValidator{
		errors: make([]string, 0),
	}
}

// AddError adds a validation error to the error list
func (v *RequestValidator) AddError(field string, message string) {
	v.errors = append(v.errors, fmt.Sprintf("%s: %s", field, message))
}

// HasErrors returns true if there are validation errors
func (v *RequestValidator) HasErrors() bool {
	return len(v.errors) > 0
}

// GetErrors returns the list of validation errors
func (v *RequestValidator) GetErrors() []string {
	return v.errors
}

// GetErrorString returns all errors as a formatted string
func (v *RequestValidator) GetErrorString() string {
	if len(v.errors) == 0 {
		return ""
	}
	return strings.Join(v.errors, "; ")
}

// ClearErrors clears all validation errors
func (v *RequestValidator) ClearErrors() {
	v.errors = make([]string, 0)
}

// ValidateAnnotationNote validates annotation note size
func (v *RequestValidator) ValidateAnnotationNote(note string) bool {
	if len(note) > MaxAnnotationNoteSize {
		v.AddError("note", fmt.Sprintf("exceeds maximum size of %d characters (got %d)", MaxAnnotationNoteSize, len(note)))
		return false
	}
	return true
}

// ValidateAnnotationColor validates annotation color size
func (v *RequestValidator) ValidateAnnotationColor(color string) bool {
	if len(color) > MaxAnnotationColorSize {
		v.AddError("color", fmt.Sprintf("exceeds maximum size of %d characters (got %d)", MaxAnnotationColorSize, len(color)))
		return false
	}
	return true
}

// ValidateWorkspaceName validates workspace name size
func (v *RequestValidator) ValidateWorkspaceName(name string) bool {
	if len(name) > MaxWorkspaceNameSize {
		v.AddError("name", fmt.Sprintf("exceeds maximum size of %d characters (got %d)", MaxWorkspaceNameSize, len(name)))
		return false
	}
	return true
}

// ValidateFilePath validates file path size
func (v *RequestValidator) ValidateFilePath(path string) bool {
	if len(path) > MaxFilePathSize {
		v.AddError("file_path", fmt.Sprintf("exceeds maximum size of %d characters (got %d)", MaxFilePathSize, len(path)))
		return false
	}
	return true
}

// ValidateJPath validates jpath size
func (v *RequestValidator) ValidateJPath(jpath string) bool {
	if len(jpath) > MaxJPathSize {
		v.AddError("jpath", fmt.Sprintf("exceeds maximum size of %d characters (got %d)", MaxJPathSize, len(jpath)))
		return false
	}
	return true
}

// ValidateID validates ID size
func (v *RequestValidator) ValidateID(id string, fieldName string) bool {
	if len(id) > MaxIDSize {
		v.AddError(fieldName, fmt.Sprintf("exceeds maximum size of %d characters (got %d)", MaxIDSize, len(id)))
		return false
	}
	return true
}

// ValidateHash validates hash size and format
func (v *RequestValidator) ValidateHash(hash string, fieldName string) bool {
	// Check size
	if len(hash) > MaxHashSize {
		v.AddError(fieldName, fmt.Sprintf("exceeds maximum size of %d characters (got %d)", MaxHashSize, len(hash)))
		return false
	}

	// Check format (hex or base64)
	if !v.ValidateHashFormat(hash) {
		v.AddError(fieldName, "must be a valid hex or base64 hash")
		return false
	}

	return true
}

// ValidateHashFormat validates if a string is a valid hash format (hex or base64)
func (v *RequestValidator) ValidateHashFormat(hash string) bool {
	// Check if it's a valid hex hash
	if isHexHash(hash) {
		return true
	}

	// Check if it's a valid base64 hash
	if isBase64Hash(hash) {
		return true
	}

	return false
}

// ValidateUUIDFormat validates if a string is a valid UUID format
func (v *RequestValidator) ValidateUUIDFormat(id string) bool {
	uuidRegex := regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
	return uuidRegex.MatchString(id)
}

// ValidateIDWithPrefix validates ID with prefix (like ann_, ws_, etc.)
func (v *RequestValidator) ValidateIDWithPrefix(id string, fieldName string) bool {
	if len(id) > MaxIDSize {
		v.AddError(fieldName, fmt.Sprintf("exceeds maximum size of %d characters (got %d)", MaxIDSize, len(id)))
		return false
	}

	// Extract UUID part if there's a prefix
	parts := strings.SplitN(id, "_", 2)
	if len(parts) == 2 {
		uuidPart := parts[1]
		if !v.ValidateUUIDFormat(uuidPart) {
			v.AddError(fieldName, "must have a valid UUID format after prefix")
			return false
		}
	} else {
		// No prefix, validate as UUID directly
		if !v.ValidateUUIDFormat(id) {
			v.AddError(fieldName, "must be a valid UUID or have prefix with UUID")
			return false
		}
	}

	return true
}

// ValidateCreateAnnotationRequest validates a CreateAnnotationRequest
func (v *RequestValidator) ValidateCreateAnnotationRequest(req CreateAnnotationRequest) bool {
	v.ClearErrors()

	// Validate workspace ID
	if !v.ValidateIDWithPrefix(req.WorkspaceID, "workspace_id") {
		return false
	}

	// Validate file hash
	if !v.ValidateHash(req.FileHash, "file_hash") {
		return false
	}

	// Validate note
	if !v.ValidateAnnotationNote(req.Note) {
		return false
	}

	// Validate color
	if !v.ValidateAnnotationColor(string(req.Color)) {
		return false
	}

	// Validate jpath
	if !v.ValidateJPath(req.Options.JPath) {
		return false
	}

	return !v.HasErrors()
}

// ValidateUpdateAnnotationRequest validates an UpdateAnnotationRequest
func (v *RequestValidator) ValidateUpdateAnnotationRequest(req UpdateAnnotationRequest) bool {
	v.ClearErrors()

	// Validate annotation ID
	if !v.ValidateIDWithPrefix(req.AnnotationID, "annotation_id") {
		return false
	}

	// Validate note if provided
	if req.Note != "" && !v.ValidateAnnotationNote(req.Note) {
		return false
	}

	// Validate color if provided
	if req.Color != "" && !v.ValidateAnnotationColor(string(req.Color)) {
		return false
	}

	return !v.HasErrors()
}

// ValidateCreateWorkspaceRequest validates a CreateWorkspaceRequest
func (v *RequestValidator) ValidateCreateWorkspaceRequest(req CreateWorkspaceRequest) bool {
	v.ClearErrors()

	// Validate name
	if !v.ValidateWorkspaceName(req.Name) {
		return false
	}

	return !v.HasErrors()
}

// ValidateUpdateWorkspaceRequest validates an UpdateWorkspaceRequest
func (v *RequestValidator) ValidateUpdateWorkspaceRequest(req UpdateWorkspaceRequest) bool {
	v.ClearErrors()

	// Validate name
	if !v.ValidateWorkspaceName(req.Name) {
		return false
	}

	return !v.HasErrors()
}

// ValidateCreateFileRequest validates a CreateFileRequest
func (v *RequestValidator) ValidateCreateFileRequest(req CreateFileRequest) bool {
	v.ClearErrors()

	// Validate workspace ID
	if !v.ValidateIDWithPrefix(req.WorkspaceID, "workspace_id") {
		return false
	}

	// Validate file hash
	if !v.ValidateHash(req.FileHash, "file_hash") {
		return false
	}

	// Validate jpath
	if !v.ValidateJPath(req.Options.JPath) {
		return false
	}

	// Validate file path if provided
	if req.FilePath != "" && !v.ValidateFilePath(req.FilePath) {
		return false
	}

	// Validate instance ID if provided
	if req.InstanceID != "" && !v.ValidateID(req.InstanceID, "instance_id") {
		return false
	}

	// Validate description if provided
	if req.Description != "" && len(req.Description) > MaxFilePathSize {
		v.AddError("description", fmt.Sprintf("exceeds maximum size of %d characters (got %d)", MaxFilePathSize, len(req.Description)))
		return false
	}

	return !v.HasErrors()
}

// ValidateUpdateFileRequest validates an UpdateFileRequest
func (v *RequestValidator) ValidateUpdateFileRequest(req UpdateFileRequest) bool {
	v.ClearErrors()

	// Validate file hash
	if !v.ValidateHash(req.FileHash, "file_hash") {
		return false
	}

	// Validate jpath from FileOptions
	if !v.ValidateJPath(req.Options.JPath) {
		return false
	}

	// Validate description if provided
	if req.Description != "" && len(req.Description) > MaxFilePathSize {
		v.AddError("description", fmt.Sprintf("exceeds maximum size of %d characters (got %d)", MaxFilePathSize, len(req.Description)))
		return false
	}

	return !v.HasErrors()
}

// ValidateDeleteFileRequest validates a DeleteFileRequest
func (v *RequestValidator) ValidateDeleteFileRequest(req DeleteFileRequest) bool {
	v.ClearErrors()

	// Validate file hash
	if !v.ValidateHash(req.FileHash, "file_hash") {
		return false
	}

	// Validate jpath from FileOptions
	if !v.ValidateJPath(req.Options.JPath) {
		return false
	}

	return !v.HasErrors()
}

// Helper functions

// isHexHash checks if a string is a valid hex hash
func isHexHash(s string) bool {
	hexRegex := regexp.MustCompile(`^[a-fA-F0-9]+$`)
	return hexRegex.MatchString(s) && (len(s) == 32 || len(s) == 40 || len(s) == 64 || len(s) == 128)
}

// isBase64Hash checks if a string is a valid base64 hash
func isBase64Hash(s string) bool {
	base64Regex := regexp.MustCompile(`^[A-Za-z0-9+/]+$`)
	return base64Regex.MatchString(s) && len(s) >= 16 && len(s) <= 88
}
