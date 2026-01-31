package plugin

import (
	"errors"
	"fmt"
	"strings"
)

// PluginManifest represents the parsed plugin.yml manifest file
type PluginManifest struct {
	ID          string   `yaml:"id"` // Unique plugin identifier (UUID)
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Description string   `yaml:"description"`
	Executable  string   `yaml:"executable"`
	Extensions  []string `yaml:"extensions"`
	Author      string   `yaml:"author"`
}

// validateManifest validates the plugin manifest fields
func validateManifest(manifest *PluginManifest) error {
	// Check required fields
	if manifest.ID == "" {
		return errors.New("manifest missing required field: id (UUID)")
	}
	// Validate UUID format (basic validation)
	if !isValidUUID(manifest.ID) {
		return fmt.Errorf("manifest field 'id' must be a valid UUID, got: %s", manifest.ID)
	}

	if manifest.Name == "" {
		return errors.New("manifest missing required field: name")
	}
	if len(manifest.Name) > 100 {
		return errors.New("manifest field 'name' exceeds 100 characters")
	}

	if manifest.Version == "" {
		return errors.New("manifest missing required field: version")
	}
	// Basic semver validation (simple pattern)
	if !isValidSemver(manifest.Version) {
		return fmt.Errorf("manifest field 'version' must be in semver format (e.g., '1.0.0'), got: %s", manifest.Version)
	}

	if manifest.Executable == "" {
		return errors.New("manifest missing required field: executable")
	}

	if len(manifest.Extensions) == 0 {
		return errors.New("manifest missing required field: extensions (must have at least one)")
	}

	// Validate description length
	if len(manifest.Description) > 500 {
		return errors.New("manifest field 'description' exceeds 500 characters")
	}

	// Validate author length
	if len(manifest.Author) > 200 {
		return errors.New("manifest field 'author' exceeds 200 characters")
	}

	// Validate extensions format
	for i, ext := range manifest.Extensions {
		if !strings.HasPrefix(ext, ".") {
			return fmt.Errorf("extension at index %d must start with a dot, got: %s", i, ext)
		}
	}

	return nil
}

// isValidUUID checks if a string is a valid UUID format
func isValidUUID(id string) bool {
	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (8-4-4-4-12)
	if len(id) != 36 {
		return false
	}
	// Check hyphens in correct positions
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return false
	}
	// Check all other characters are hex digits
	for i, ch := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue // skip hyphens
		}
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}

// isValidSemver checks if a version string follows basic semver format
func isValidSemver(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	// Each part should be numeric (simple check)
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	return true
}
