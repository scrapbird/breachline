# Custom File Loader Plugin System

## Overview

This document describes the implementation plan for adding custom file loader plugin support to BreachLine. This feature allows users to load any file type (custom logs, binary formats, etc.) by installing executable-based plugins written in any language.

## Requirements

### Functional Requirements
- Support any file type via external executable plugins
- Plugins written in any language (Python, Go, Rust, etc.)
- Plugins register supported file extensions via plugin.yml manifest
- User-installed plugins (not bundled with app)
- Plugin management UI in settings dialog
- Plugin output cached same as CSV/JSON/XLSX
- Cross-platform: Windows, macOS, Linux

### Non-Requirements (Future)
- Per-plugin configuration
- Streaming support (can load entire output to memory)
- Plugin signing/verification
- Bundled plugins

## Architecture

### High-Level Design

**Plugin Execution Model**: Subprocess-based
- App spawns plugin executable as subprocess
- Communication via command-line args + stdout
- Plugin outputs CSV format (most efficient cross-process tabular data)
- Plugin lifecycle: discover → validate → execute → cache

**Integration Points**:
1. `fileloader` package - plugin registry & execution
2. `settings` package - plugin configuration storage
3. Frontend settings dialog - plugin management UI
4. File type detection - check plugin registry before defaulting

### Component Architecture

```
┌─────────────────────────────────────────────────────┐
│                  BreachLine App                     │
│                                                     │
│  ┌───────────────────────────────────────────────┐ │
│  │           Plugin Registry                     │ │
│  │  - Discover plugins from settings             │ │
│  │  - Map extensions → plugin executable         │ │
│  │  - Validate plugin.yml manifests              │ │
│  └───────────────────────────────────────────────┘ │
│                        │                            │
│  ┌───────────────────────────────────────────────┐ │
│  │          Plugin Executor                      │ │
│  │  - Spawn subprocess with --mode flag          │ │
│  │  - Parse CSV output from stdout               │ │
│  │  - Handle errors from stderr                  │ │
│  └───────────────────────────────────────────────┘ │
│                        │                            │
│  ┌───────────────────────────────────────────────┐ │
│  │      FileLoader Integration                   │ │
│  │  - DetectFileType() checks plugin registry    │ │
│  │  - proxy.go dispatches to plugin loader       │ │
│  │  - Cache plugin output as Row objects         │ │
│  └───────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
                        │
                        │ subprocess exec
                        ▼
┌─────────────────────────────────────────────────────┐
│              Plugin Executable                      │
│  (Python, Go, Rust, Shell script, etc.)            │
│                                                     │
│  Modes:                                             │
│  --mode=header  → Output CSV header row            │
│  --mode=count   → Output row count (single int)    │
│  --mode=stream  → Output full CSV (header + rows)  │
└─────────────────────────────────────────────────────┘
```

## Implementation Plan

### Phase 1: Core Plugin Infrastructure

#### 1.1 Settings Schema Extension
**File**: `application/app/settings/types.go`

Add to Settings struct:
```go
// Plugin loader settings
EnablePlugins bool             `yaml:"enable_plugins" json:"enable_plugins"`
Plugins       []PluginConfig   `yaml:"plugins,omitempty" json:"plugins,omitempty"`
```

New types:
```go
type PluginConfig struct {
    ID          string   `yaml:"id" json:"id"`                   // Unique plugin identifier (UUID from plugin.yml)
    Name        string   `yaml:"name" json:"name"`               // Display name
    Enabled     bool     `yaml:"enabled" json:"enabled"`         // Enable/disable toggle
    Path        string   `yaml:"path" json:"path"`               // Absolute path to plugin directory
    Extensions  []string `yaml:"extensions" json:"extensions"`   // Cached from plugin.yml
    Description string   `yaml:"description" json:"description"` // Cached from plugin.yml
}
```

Default value:
```go
EnablePlugins: false, // Disabled by default
Plugins:       []PluginConfig{},
```

#### 1.2 Plugin Manifest Definition
**File**: `doc/PLUGIN_MANIFEST_SPEC.md` (new)

Plugin directory structure:
```
<plugin-directory>/
  plugin.yml          # Manifest file
  <executable>        # Plugin binary/script
```

`plugin.yml` format:
```yaml
id: f47ac10b-58cc-4372-a567-0e02b2c3d479  # Required: Unique UUID for this plugin
name: Parquet Loader
version: 1.0.0
description: Load Apache Parquet files
executable: parquet-loader  # Relative to plugin.yml, or absolute path
extensions:
  - .parquet
  - .pq
author: Your Name
```

**Validation rules**:
- `id` required, must be valid UUID format (8-4-4-4-12)
- `name` required, max 100 chars
- `version` required, semver format
- `description` optional, max 500 chars
- `executable` required, must exist relative to plugin.yml
- `extensions` required, min 1 extension, each starts with "."

#### 1.3 Plugin Registry
**File**: `application/app/fileloader/plugin_registry.go` (new)

```go
package fileloader

type PluginManifest struct {
    ID          string   // Unique plugin identifier (UUID)
    Name        string
    Version     string
    Description string
    Executable  string
    Extensions  []string
    Author      string
}

type PluginRegistry struct {
    plugins     map[string]*PluginInfo // extension → plugin
    mu          sync.RWMutex
}

type PluginInfo struct {
    Config   settings.PluginConfig
    Manifest PluginManifest
    ExecPath string // Resolved absolute path to executable
}

func NewPluginRegistry() *PluginRegistry
func (r *PluginRegistry) LoadFromSettings(configs []settings.PluginConfig) error
func (r *PluginRegistry) GetPluginForExtension(ext string) (*PluginInfo, bool)
func (r *PluginRegistry) ListPlugins() []*PluginInfo
func (r *PluginRegistry) ValidatePlugin(path string) (*PluginManifest, error)
```

Key behaviors:
- Parse plugin.yml using `gopkg.in/yaml.v3`
- Validate executable exists and is executable (os.Stat + check permissions)
- Extension conflicts: last plugin wins (warn user in logs)
- Re-scan when settings change

#### 1.4 Plugin Executor
**File**: `application/app/fileloader/plugin_executor.go` (new)

```go
package fileloader

type PluginExecutor struct {
    plugin *PluginInfo
}

type PluginMode string
const (
    PluginModeHeader PluginMode = "header"
    PluginModeCount  PluginMode = "count"
    PluginModeStream PluginMode = "stream"
)

func NewPluginExecutor(plugin *PluginInfo) *PluginExecutor
func (e *PluginExecutor) Execute(ctx context.Context, mode PluginMode, filePath string, options FileOptions) ([]byte, error)
func (e *PluginExecutor) ReadHeader(ctx context.Context, filePath string, options FileOptions) ([]string, error)
func (e *PluginExecutor) GetRowCount(ctx context.Context, filePath string, options FileOptions) (int, error)
func (e *PluginExecutor) GetReader(ctx context.Context, filePath string, options FileOptions) (*csv.Reader, error)
```

Command format:
```bash
<plugin-exec> --mode=header --file=/path/to/file.ext
<plugin-exec> --mode=count --file=/path/to/file.ext
<plugin-exec> --mode=stream --file=/path/to/file.ext
```

Expected output:
- **header mode**: CSV header row (single line)
- **count mode**: Single integer (row count, excluding header)
- **stream mode**: Full CSV output (header + data rows)

Error handling:
- Non-zero exit code → return error with stderr contents
- Context cancellation → kill plugin subprocess, return cancellation error
- Parse errors → return descriptive error
- Log all plugin execution to app logs

### Phase 2: FileLoader Integration

#### 2.1 Global Plugin Registry
**File**: `application/app/fileloader/plugin.go` (new)

```go
package fileloader

var (
    globalPluginRegistry *PluginRegistry
    pluginRegistryMu     sync.RWMutex
)

func SetPluginRegistry(registry *PluginRegistry)
func GetPluginRegistry() *PluginRegistry
func InitializePluginRegistry(configs []settings.PluginConfig) error
```

#### 2.2 File Type Detection Enhancement
**File**: `application/app/fileloader/detection.go`

Modify `DetectFileType()`:
```go
func DetectFileType(filePath string) FileType {
    if filePath == "" {
        return FileTypeUnknown
    }

    ext := strings.ToLower(filepath.Ext(filePath))

    // Check built-in types first
    switch ext {
    case ".csv":
        return FileTypeCSV
    case ".xlsx":
        return FileTypeXLSX
    case ".json":
        return FileTypeJSON
    }

    // Check plugin registry
    if registry := GetPluginRegistry(); registry != nil {
        if plugin, ok := registry.GetPluginForExtension(ext); ok {
            return FileTypePlugin // New type
        }
    }

    // Default to CSV for backwards compatibility
    return FileTypeCSV
}
```

Add new FileType:
```go
const (
    FileTypeUnknown FileType = iota
    FileTypeCSV
    FileTypeXLSX
    FileTypeJSON
    FileTypePlugin // NEW
)
```

#### 2.3 Plugin File Loader
**File**: `application/app/fileloader/plugin_loader.go` (new)

Implement plugin-specific functions matching CSV/JSON/XLSX patterns:
```go
func ReadPluginHeader(ctx context.Context, filePath string, options FileOptions) ([]string, error)
func ReadPluginHeaderFromBytes(data []byte, options FileOptions) ([]string, error)
func GetPluginRowCount(ctx context.Context, filePath string, options FileOptions) (int, error)
func GetPluginRowCountFromBytes(data []byte, options FileOptions) (int, error)
func GetPluginReader(ctx context.Context, filePath string, options FileOptions) (*csv.Reader, *os.File, error)
func GetPluginReaderFromBytes(data []byte, options FileOptions) (*csv.Reader, error)
```

Notes:
- FromBytes variants not supported for plugins (no compression support)
- Return errors for FromBytes calls
- Cache plugin execution results at Row level (reuse JSON caching pattern)
- Context passed from caller allows cancellation during long plugin execution

#### 2.4 Proxy Function Updates
**File**: `application/app/fileloader/proxy.go`

Add FileTypePlugin cases to:
- `ReadHeaderWithOptions()`
- `GetRowCountWithOptions()`
- `GetReader()`

Example:
```go
case FileTypePlugin:
    return ReadPluginHeader(filePath, options)
```

#### 2.5 Plugin Caching
**Strategy**: Reuse existing Row-based caching (same as JSON)

Cache key format:
```
plugin:<plugin-id>:<file-hash>::time:<timeIdx>::tz:<timezone>
```

The plugin ID (UUID from plugin.yml) is used instead of the plugin path for cache keys, ensuring cache stability even if the plugin is moved to a different location.

Implementation:
- Store plugin output as `[]*interfaces.Row` in base data cache
- Cache key includes file hash, so any file changes automatically cause cache miss
- Invalidate on plugin config change (track plugin version in cache key)

### Phase 3: Settings UI

#### 3.1 Backend API Additions
**File**: `application/app/settings/service.go`

New methods:
```go
func (s *Service) AddPlugin(path string) (*PluginConfig, error)
func (s *Service) RemovePlugin(path string) error
func (s *Service) TogglePlugin(path string, enabled bool) error
func (s *Service) ValidatePluginPath(path string) (*PluginManifest, error)
func (s *Service) GetPlugins() []PluginConfig
```

Workflow for AddPlugin:
1. Validate path exists
2. Look for plugin.yml in same directory as executable
3. Parse and validate plugin.yml
4. Create PluginConfig with cached manifest data
5. Add to settings.Plugins
6. Save settings
7. Reinitialize plugin registry

**File**: `application/app/app.go`

Add Wails bindings:
```go
func (a *App) AddPlugin(path string) (*PluginConfig, error)
func (a *App) RemovePlugin(path string) error
func (a *App) TogglePlugin(path string, enabled bool) error
func (a *App) ValidatePluginPath(path string) (*PluginManifest, error)
func (a *App) GetPlugins() []PluginConfig
```

#### 3.2 Frontend Settings Dialog Restructure
**Goal**: Multi-tab settings dialog with General + Plugins tabs

**Files to modify**:
- `application/frontend/src/components/SettingsDialog.tsx`

Current structure:
```tsx
<Dialog>
  <DialogContent>
    {/* All settings in single scrollable area */}
  </DialogContent>
</Dialog>
```

New structure:
```tsx
<Dialog>
  <Tabs defaultValue="general">
    <TabsList>
      <TabsTrigger value="general">General</TabsTrigger>
      <TabsTrigger value="plugins">Plugins</TabsTrigger>
    </TabsList>

    <TabsContent value="general">
      {/* Existing settings moved here */}
    </TabsContent>

    <TabsContent value="plugins">
      {/* New plugin management UI */}
    </TabsContent>
  </Tabs>
</Dialog>
```

UI components needed:
- Use shadcn/ui Tabs component (already available)
- Move all existing settings to "General" tab
- Create new "Plugins" tab component

#### 3.3 Plugin Management UI Component
**File**: `application/frontend/src/components/PluginSettingsTab.tsx` (new)

UI Layout:
```
┌─────────────────────────────────────────────────┐
│ Plugins                                         │
│                                                 │
│ ☐ Enable plugin support                        │
│                                                 │
│ Installed Plugins:                              │
│                                                 │
│ ┌─────────────────────────────────────────────┐│
│ │ ☑ Parquet Loader                     [Edit] ││
│ │   Extensions: .parquet, .pq                 ││
│ │   /home/user/plugins/parquet-loader         ││
│ ├─────────────────────────────────────────────┤│
│ │ ☐ Custom Log Parser                  [Edit] ││
│ │   Extensions: .log                          ││
│ │   /usr/local/bin/log-parser                 ││
│ └─────────────────────────────────────────────┘│
│                                                 │
│ [+ Add Plugin]                                  │
└─────────────────────────────────────────────────┘
```

State management:
```typescript
interface PluginConfig {
  name: string;
  enabled: boolean;
  path: string;
  extensions: string[];
  description: string;
}

const [enablePlugins, setEnablePlugins] = useState(false);
const [plugins, setPlugins] = useState<PluginConfig[]>([]);
```

Actions:
- **Add Plugin**:
  - Open file picker (Wails runtime.OpenFileDialog)
  - Call `ValidatePluginPath(path)` to get manifest
  - Show confirmation dialog with plugin details
  - Call `AddPlugin(path)` to save
  - Refresh plugin list

- **Toggle Plugin**:
  - Call `TogglePlugin(path, enabled)`
  - Update local state

- **Remove Plugin**:
  - Show confirmation dialog
  - Call `RemovePlugin(path)`
  - Refresh plugin list

- **Edit Plugin**:
  - Not implemented in Phase 1 (future: allow changing path)

Validation:
- Show error if plugin.yml not found
- Show error if executable not found
- Show error if extension conflicts with existing plugin
- Show warning if plugin executable not marked as executable

#### 3.4 Wails TypeScript Bindings
**File**: `application/frontend/wailsjs/go/app/App.d.ts` (auto-generated)

Bindings will be auto-generated after adding Go methods. Types:
```typescript
export function AddPlugin(path: string): Promise<settings.PluginConfig>;
export function RemovePlugin(path: string): Promise<void>;
export function TogglePlugin(path: string, enabled: boolean): Promise<void>;
export function ValidatePluginPath(path: string): Promise<fileloader.PluginManifest>;
export function GetPlugins(): Promise<settings.PluginConfig[]>;
```

### Phase 4: Testing & Documentation

#### 4.1 Example Plugin
**File**: `tools/example-plugin/` (new directory)

Create simple Python example plugin:
```
tools/example-plugin/
  plugin.yml
  example-loader
  example-loader.py (symlink or wrapper script)
  README.md
```

Example: CSV uppercase converter (converts all text to uppercase)
- Reads any file
- Outputs CSV with single column "content"
- Each row is one line from file (uppercased)

Purpose: Test and demonstrate plugin API

#### 4.2 Plugin Developer Documentation
**File**: `doc/PLUGIN_DEVELOPER_GUIDE.md` (new)

Contents:
- Plugin manifest specification
- Command-line interface requirements
- Expected output formats for each mode
- Error handling best practices
- Example plugins in Python, Go, Shell script
- Testing plugins standalone before installation

#### 4.3 User Documentation
**File**: `application/frontend/src/components/PluginHelp.tsx` (new)

Add help dialog linked from Plugins tab with:
- How to find/install plugins
- How to create plugin.yml
- Security considerations (unsigned plugins)
- Troubleshooting common issues

#### 4.4 Integration Tests
**File**: `application/app/fileloader/plugin_integration_test.go` (new)

Test scenarios:
- Load simple plugin, execute all modes
- Test extension detection
- Test caching behavior
- Test error handling (bad executable, timeout, invalid output)
- Test plugin enable/disable
- Test extension conflicts

### Phase 5: Polish & Edge Cases

#### 5.1 Error Handling
- Plugin executable not found → clear error message
- Plugin timeout → "Plugin did not respond within 30 seconds"
- Invalid CSV output → show first 500 chars of output in error
- Plugin.yml parse error → show YAML line number
- Extension conflict → warn in logs, last plugin wins

#### 5.2 Logging
Add detailed logging to:
- Plugin discovery (`Loaded plugin X with extensions Y`)
- Plugin execution (`Executing plugin X in mode Y for file Z`)
- Plugin errors (`Plugin X failed: stderr`)
- Cache hits/misses for plugin data

#### 5.3 Performance Considerations
- Cache plugin registry in memory (don't re-parse plugin.yml on every file load)
- Invalidate registry cache when settings change
- Use Row-based caching to avoid re-executing plugins
- Plugin execution can take as long as needed (no timeout)
- Use context cancellation to allow user to cancel long-running plugins

#### 5.4 Security Considerations
Document in user-facing help:
- Plugins run with full user permissions
- No sandboxing (future enhancement)
- User responsible for trusting plugin sources
- Recommend reviewing plugin code before installing
- Plugins can read any file accessible to BreachLine

#### 5.5 Future Enhancements (Not in Scope)
- Per-plugin configuration (custom flags)
- Streaming support for large files
- Plugin marketplace/repository
- Plugin signing/verification
- Sandboxed plugin execution
- Plugin SDK in multiple languages
- Bundled plugins (shipped with app)
- Plugin auto-updates
- Progress reporting from plugins
- Bidirectional communication (not just stdout)

## File Structure Changes

### New Files
```
application/app/fileloader/
  plugin.go                  # Global registry management
  plugin_registry.go         # Registry implementation
  plugin_executor.go         # Plugin subprocess execution
  plugin_loader.go           # Plugin fileloader functions
  plugin_integration_test.go # Integration tests

application/frontend/src/components/
  PluginSettingsTab.tsx      # Plugin management UI
  PluginHelp.tsx            # User help dialog

doc/
  PLUGIN_SYSTEM.md          # This file
  PLUGIN_MANIFEST_SPEC.md   # Plugin.yml specification
  PLUGIN_DEVELOPER_GUIDE.md # Plugin developer docs

tools/example-plugin/
  plugin.yml
  example-loader
  example-loader.py
  README.md
```

### Modified Files
```
application/app/settings/
  types.go                  # Add PluginConfig, EnablePlugins
  service.go                # Add plugin management methods

application/app/
  app.go                    # Add Wails bindings for plugins

application/app/fileloader/
  types.go                  # Add FileTypePlugin
  detection.go              # Check plugin registry
  proxy.go                  # Dispatch to plugin loader

application/frontend/src/components/
  SettingsDialog.tsx        # Restructure with tabs
```

## Implementation Checklist

### Backend (Go)
- [ ] Add settings schema (PluginConfig, EnablePlugins)
- [ ] Implement PluginRegistry (load, validate, lookup)
- [ ] Implement PluginExecutor (subprocess execution)
- [ ] Add plugin fileloader functions (header, count, reader)
- [ ] Integrate with DetectFileType and proxy functions
- [ ] Add Wails bindings (AddPlugin, RemovePlugin, etc.)
- [ ] Implement plugin caching (reuse Row cache)
- [ ] Add plugin logging throughout
- [ ] Write integration tests

### Frontend (TypeScript/React)
- [ ] Restructure SettingsDialog with tabs
- [ ] Move existing settings to General tab
- [ ] Create PluginSettingsTab component
- [ ] Implement Add Plugin flow (file picker + validation)
- [ ] Implement Toggle Plugin (enable/disable)
- [ ] Implement Remove Plugin (with confirmation)
- [ ] Add PluginHelp dialog
- [ ] Style plugin list (match existing UI patterns)
- [ ] Handle error states (validation errors, etc.)

### Documentation
- [ ] Write PLUGIN_MANIFEST_SPEC.md
- [ ] Write PLUGIN_DEVELOPER_GUIDE.md
- [ ] Create example plugin (Python)
- [ ] Add plugin help content

### Testing
- [ ] Test plugin discovery and validation
- [ ] Test plugin execution (all modes)
- [ ] Test file type detection with plugins
- [ ] Test caching behavior
- [ ] Test error handling (bad plugins)
- [ ] Test UI flows (add/remove/toggle)
- [ ] Test cross-platform (Windows/Mac/Linux)

## Dependencies

### Go Packages
- `gopkg.in/yaml.v3` (YAML parsing) - already in project
- `os/exec` (subprocess) - standard library

### Frontend
- `@radix-ui/react-tabs` (via shadcn/ui) - likely already available
- No new npm dependencies required

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Plugin crashes app | High | Subprocess isolation prevents crashes |
| Slow plugins block UI | Medium | Context cancellation allows user to cancel |
| Malicious plugins | High | Document security, no sandboxing (future) |
| CSV parsing errors | Medium | Strict validation, clear error messages |
| Extension conflicts | Low | Last wins, warn in logs |
| Cross-platform path issues | Medium | Use filepath package, test on all platforms |
| Plugin.yml not found | Low | Clear validation errors in UI |

## Success Criteria

- [ ] User can add plugin via settings UI
- [ ] Plugin-loaded files appear in grid same as CSV
- [ ] Plugin output cached and reused
- [ ] Plugin errors shown clearly to user
- [ ] Works on Windows, macOS, Linux
- [ ] Example plugin provided and tested
- [ ] Documentation complete for plugin developers
- [ ] No performance regression for built-in types

## Timeline Estimate

- **Phase 1** (Core Infrastructure): 2-3 days
- **Phase 2** (FileLoader Integration): 1-2 days
- **Phase 3** (Settings UI): 2-3 days
- **Phase 4** (Testing & Docs): 1-2 days
- **Phase 5** (Polish): 1 day

**Total**: ~7-11 days of development time

## Questions & Decisions

### Resolved
- ✅ Plugin format: YAML (plugin.yml)
- ✅ Plugin API: Executable subprocess
- ✅ Output format: CSV
- ✅ Plugin location: User-specified path
- ✅ Extension conflicts: Last wins
- ✅ UI location: New tab in settings dialog
- ✅ Plugin timeout: No timeout, use context cancellation for user-initiated cancel

### Open
- Should we validate CSV output schema? (Proposed: No, trust plugin)
- Max plugin output size? (Proposed: No limit, use streaming CSV reader)
- Should disabled plugins still appear in registry? (Proposed: Yes, for UI display)
