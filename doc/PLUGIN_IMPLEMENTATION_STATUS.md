# Plugin System Implementation Status

## Overview

The BreachLine plugin system has been implemented with full backend support. Users can now create custom file loaders to handle any file format by writing executables that convert their data to CSV.

## ✅ Completed Components

### Backend Infrastructure (Phase 1)

- **Settings Schema** (`app/settings/types.go`)
  - `PluginConfig` type for plugin metadata
  - `EnablePlugins` and `Plugins` fields in Settings
  - Full YAML serialization support

- **Plugin Manifest** (`doc/PLUGIN_MANIFEST_SPEC.md`)
  - Complete specification for `plugin.yml` format
  - Validation rules and examples
  - Platform considerations documented

- **Plugin Registry** (`app/fileloader/plugin_registry.go`)
  - Discovers and loads plugins from settings
  - Validates plugin manifests and executables
  - Maps file extensions to plugins
  - Handles extension conflicts (last wins)

- **Plugin Executor** (`app/fileloader/plugin_executor.go`)
  - Spawns plugin subprocess with proper arguments
  - Supports three execution modes: header, count, stream
  - Parses CSV output from plugins
  - Comprehensive error handling

### FileLoader Integration (Phase 2)

- **File Type Detection** (`app/fileloader/detection.go`)
  - Added `FileTypePlugin` constant
  - Checks plugin registry for unknown extensions
  - Falls back to CSV for backwards compatibility

- **Plugin Loader** (`app/fileloader/plugin_loader.go`)
  - `ReadPluginHeader()` - reads CSV headers
  - `GetPluginRowCount()` - gets row count
  - `GetPluginReader()` - returns CSV reader
  - Context-aware execution (cancellable)

- **Proxy Functions** (`app/fileloader/proxy.go`)
  - Updated all dispatchers to handle FileTypePlugin
  - Integrated with existing file loading pipeline
  - Works with caching and compression

### Backend API (Phase 3)

- **Settings Service** (`app/settings/service.go`)
  - `GetPlugins()` - list all plugins
  - `AddPlugin(path)` - validate and add plugin
  - `RemovePlugin(path)` - remove plugin
  - `TogglePlugin(path, enabled)` - enable/disable plugin

- **Wails Bindings** (`app/app.go`)
  - All plugin methods exposed to frontend
  - Plugin registry initialization on startup
  - Automatic reload on plugin changes
  - `ValidatePluginPath()` for UI validation

- **Interface Types** (`app/interfaces/types.go`)
  - `PluginConfig` type for frontend
  - Added to `Settings` struct
  - JSON serialization ready

### Documentation & Examples (Phase 4)

- **Plugin Manifest Specification** (`doc/PLUGIN_MANIFEST_SPEC.md`)
  - Complete field specifications
  - Validation rules
  - Platform considerations
  - Best practices

- **Plugin Developer Guide** (`doc/PLUGIN_DEVELOPER_GUIDE.md`)
  - Comprehensive API documentation
  - Implementation examples (Python, Go, Shell)
  - Testing guide
  - Troubleshooting section
  - Security considerations

- **Example Plugin** (`tools/example-plugin/`)
  - Working text uppercaser plugin
  - Demonstrates all three modes
  - Includes README and usage instructions
  - Can be used as a template

## ⏳ Remaining Work

### Frontend UI (Not Started)

The backend is fully functional but lacks a user interface. To complete the plugin system, the following UI components need to be built:

#### 1. Settings Dialog Restructuring

**File**: `application/frontend/src/components/Settings.tsx`

**Changes Needed**:
- Restructure Settings component to use tabs (General, Plugins)
- Move existing settings to "General" tab
- Add "Plugins" tab container
- Use shadcn/ui Tabs component (or similar)

**Example Structure**:
```tsx
<Dialog>
  <Tabs defaultValue="general">
    <TabsList>
      <TabsTrigger value="general">General</TabsTrigger>
      <TabsTrigger value="plugins">Plugins</TabsTrigger>
    </TabsList>
    <TabsContent value="general">
      {/* Existing settings */}
    </TabsContent>
    <TabsContent value="plugins">
      <PluginSettingsTab />
    </TabsContent>
  </Tabs>
</Dialog>
```

#### 2. Plugin Management Component

**File**: `application/frontend/src/components/PluginSettingsTab.tsx` (new)

**Features Needed**:
- **Enable Plugins Toggle**: Master switch for plugin support
- **Plugin List**: Display all installed plugins with:
  - Plugin name and description
  - Enabled/disabled toggle
  - File extensions handled
  - Plugin path
  - Remove button
- **Add Plugin Button**: Opens file picker to select plugin executable/directory
- **Validation Feedback**: Show errors from ValidatePluginPath()
- **Confirmation Dialogs**: For removing plugins

**State Management**:
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

**Backend Calls**:
```typescript
import { GetPlugins, AddPlugin, RemovePlugin, TogglePlugin, ValidatePluginPath } from '../wailsjs/go/app/App';

// Load plugins
const plugins = await GetPlugins();

// Add plugin
const newPlugin = await AddPlugin(selectedPath);

// Remove plugin
await RemovePlugin(pluginPath);

// Toggle plugin
await TogglePlugin(pluginPath, enabled);

// Validate before adding
const manifest = await ValidatePluginPath(selectedPath);
```

#### 3. Wails TypeScript Bindings

The TypeScript bindings for the plugin methods will be auto-generated by Wails when the app is built. Expected signatures:

```typescript
// In wailsjs/go/app/App.d.ts
export function GetPlugins(): Promise<settings.PluginConfig[]>;
export function AddPlugin(path: string): Promise<settings.PluginConfig>;
export function RemovePlugin(path: string): Promise<void>;
export function TogglePlugin(path: string, enabled: boolean): Promise<void>;
export function ValidatePluginPath(path: string): Promise<fileloader.PluginManifest>;
```

## 🧪 Testing

### Backend Testing

The backend can be tested without the UI:

1. **Manually edit settings file** (`breachline.yml`):
   ```yaml
   enable_plugins: true
   plugins:
     - name: Example Uppercaser
       enabled: true
       path: /path/to/tools/example-plugin/example-loader
       extensions:
         - .example
       description: Example plugin
   ```

2. **Create test file**:
   ```bash
   echo -e "hello world\nthis is a test" > test.example
   ```

3. **Open test file in BreachLine**:
   - The plugin should automatically handle the file
   - Data should appear in the grid

### Example Plugin Testing

Test the example plugin standalone:

```bash
cd tools/example-plugin
chmod +x example-loader

# Test modes
./example-loader --mode=header --file=../../test/test.csv
./example-loader --mode=count --file=../../test/test.csv
./example-loader --mode=stream --file=../../test/test.csv
```

## 📝 Implementation Notes

### Design Decisions

1. **Subprocess-based execution**: Plugins run as separate processes for isolation and language independence
2. **CSV output format**: Simple, universal format that integrates with existing BreachLine infrastructure
3. **Extension-based routing**: File extensions determine which plugin handles a file
4. **Last-wins conflict resolution**: If multiple plugins register the same extension, the last loaded plugin wins
5. **Disabled by default**: Plugins must be explicitly enabled in settings for security

### Performance Considerations

- Plugin output is cached using the same Row-based caching as JSON files
- Cache keys include plugin path hash and file hash for proper invalidation
- Context cancellation allows users to cancel long-running plugin operations
- No timeout on plugin execution (user can cancel manually)

### Security

- Plugins run with full user permissions (no sandboxing)
- Users must trust plugin sources
- Documented in developer guide
- Future enhancement: plugin signing/verification

## 🔮 Future Enhancements

Not in current scope but documented for future consideration:

- **Plugin Configuration**: Per-plugin settings passed as additional arguments
- **Streaming Support**: For very large files, progressive output
- **Plugin Marketplace**: Centralized repository of verified plugins
- **Plugin Signing**: Cryptographic verification of plugin authenticity
- **Sandboxing**: Restrict plugin permissions
- **Plugin SDK**: Helper libraries in multiple languages
- **Bundled Plugins**: Ship BreachLine with common format plugins
- **Progress Reporting**: Plugins report progress for large files
- **Bidirectional Communication**: Plugins can query BreachLine for additional data

## 📚 Documentation Files

- `doc/PLUGIN_SYSTEM.md` - Original design document
- `doc/PLUGIN_MANIFEST_SPEC.md` - Manifest file specification
- `doc/PLUGIN_DEVELOPER_GUIDE.md` - Guide for plugin developers
- `doc/PLUGIN_IMPLEMENTATION_STATUS.md` - This file
- `tools/example-plugin/README.md` - Example plugin documentation

## ✅ Checklist for UI Completion

- [ ] Install/configure Tabs component (shadcn/ui or custom)
- [ ] Restructure Settings.tsx to use tabs
- [ ] Create PluginSettingsTab.tsx component
- [ ] Implement plugin list UI
- [ ] Add plugin add/remove/toggle functionality
- [ ] Add validation and error handling
- [ ] Add confirmation dialogs
- [ ] Style to match existing BreachLine UI
- [ ] Test with example plugin
- [ ] Test error cases (invalid plugins, missing manifests)

## Summary

The BreachLine plugin system is **fully functional** from a backend perspective. Developers can create plugins, and the system will load and execute them correctly. The only missing piece is the **Settings UI** to manage plugins through the BreachLine interface.

Until the UI is implemented, plugins can be managed by:
1. Manually editing the `breachline.yml` settings file
2. Restarting BreachLine to reload plugins

The plugin system is production-ready from an API and execution standpoint.
