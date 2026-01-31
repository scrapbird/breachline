# Plugin Manifest Specification

## Overview

This document defines the specification for BreachLine plugin manifests. A plugin manifest (`plugin.yml`) describes a plugin's metadata, capabilities, and how the BreachLine application should interact with it.

## File Structure

Every BreachLine plugin must be contained in a directory with the following structure:

```
<plugin-directory>/
  plugin.yml          # Manifest file (required)
  <executable>        # Plugin binary/script (required)
  [other files]       # Optional supporting files
```

## Manifest Format

The manifest file must be named `plugin.yml` and use YAML format.

### Required Fields

```yaml
id: string            # Unique plugin identifier (UUID format)
name: string          # Human-readable plugin name
version: string       # Semantic version (e.g., "1.0.0")
description: string   # Brief description of plugin functionality
executable: string    # Path to executable relative to plugin.yml or absolute path
extensions: []string  # List of file extensions this plugin handles
```

### Optional Fields

```yaml
author: string        # Plugin author name or organization
```

### Complete Example

```yaml
id: f47ac10b-58cc-4372-a567-0e02b2c3d479
name: Parquet Loader
version: 1.0.0
description: Load Apache Parquet files into BreachLine
executable: parquet-loader
extensions:
  - .parquet
  - .pq
author: John Doe
```

## Field Specifications

### `id` (required)
- **Type**: String
- **Format**: UUID (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
- **Description**: Unique identifier for the plugin. This ID is used internally for cache keys, annotations, and file associations. Once assigned, it should never change even if the plugin is moved to a different location.
- **Example**: `"f47ac10b-58cc-4372-a567-0e02b2c3d479"`
- **Validation**: Must be a valid UUID format (8-4-4-4-12 hexadecimal characters separated by hyphens)
- **Generation**: You can generate a UUID using:
  - Command line: `uuidgen` (Linux/macOS) or `[guid]::NewGuid()` (PowerShell)
  - Online: https://www.uuidgenerator.net/
  - Python: `import uuid; print(uuid.uuid4())`

### `name` (required)
- **Type**: String
- **Max length**: 100 characters
- **Description**: Human-readable display name for the plugin
- **Example**: `"Parquet Loader"`, `"Custom Log Parser"`

### `version` (required)
- **Type**: String
- **Format**: Semantic versioning (MAJOR.MINOR.PATCH)
- **Description**: Plugin version number
- **Example**: `"1.0.0"`, `"2.3.1"`
- **Validation**: Must match pattern `^\d+\.\d+\.\d+$`

### `description` (optional)
- **Type**: String
- **Max length**: 500 characters
- **Description**: Brief explanation of what the plugin does
- **Example**: `"Loads Apache Parquet files and converts them to tabular format"`

### `executable` (required)
- **Type**: String
- **Description**: Path to the plugin executable
- **Format**: 
  - Relative path (from plugin.yml directory): `"./parquet-loader"`, `"parquet-loader"`
  - Absolute path: `"/usr/local/bin/parquet-loader"`
- **Validation**: 
  - File must exist
  - File must have execute permissions (on Unix-like systems)

### `extensions` (required)
- **Type**: Array of strings
- **Min items**: 1
- **Description**: List of file extensions this plugin can handle
- **Format**: Each extension must start with a dot (`.`)
- **Example**: `[".parquet", ".pq"]`, `[".log"]`
- **Case sensitivity**: Extensions are case-insensitive (`.CSV` equals `.csv`)

### `author` (optional)
- **Type**: String
- **Max length**: 200 characters
- **Description**: Plugin author or organization name
- **Example**: `"John Doe"`, `"Acme Corporation"`

## Validation Rules

BreachLine performs the following validation when loading a plugin:

1. **Manifest file must exist**: `plugin.yml` must be in the same directory as the executable
2. **Valid YAML**: Manifest must be parseable as YAML
3. **Required fields present**: `id`, `name`, `version`, `executable`, `extensions` must all be present
4. **ID format**: The `id` field must be a valid UUID (8-4-4-4-12 format)
5. **ID uniqueness**: No two plugins can have the same ID
6. **Field constraints**: All field-specific constraints (length, format) must be satisfied
7. **Executable exists**: The file referenced by `executable` must exist
8. **Executable permissions**: On Unix-like systems, the executable must have execute permission
9. **Extension format**: All extensions must start with a dot

## Error Handling

If validation fails, BreachLine will:
- Refuse to load the plugin
- Display a clear error message indicating which validation rule failed
- Log the error details for debugging

## Extension Conflicts

If multiple plugins register the same file extension:
- The **last plugin loaded** takes precedence
- A warning is logged to help users identify the conflict
- Users can control load order by disabling conflicting plugins

## Platform Considerations

### Windows
- Executable can be `.exe`, `.bat`, `.cmd`, or any executable file
- Executable permissions are not validated (Windows handles this differently)
- Paths use Windows path separators (`\`), but forward slashes (`/`) are also supported

### macOS / Linux
- Executable must have execute permission (`chmod +x`)
- Scripts should include shebang line (e.g., `#!/usr/bin/env python3`)
- Paths use Unix path separators (`/`)

## Best Practices

1. **Versioning**: Use semantic versioning and increment appropriately
2. **Descriptions**: Write clear, concise descriptions of what your plugin does
3. **Extensions**: Only register extensions your plugin can actually handle
4. **Executable naming**: Use descriptive names without spaces (e.g., `parquet-loader` not `parquet loader`)
5. **Cross-platform**: If supporting multiple platforms, consider platform-specific executables with conditional logic

## Future Enhancements

The following fields may be added in future versions:

- `config_schema`: JSON schema for plugin-specific configuration
- `min_breachline_version`: Minimum BreachLine version required
- `homepage`: URL to plugin documentation or repository
- `license`: Plugin license identifier (e.g., "MIT", "Apache-2.0")
- `dependencies`: List of system dependencies required by the plugin

## See Also

- [Plugin Developer Guide](PLUGIN_DEVELOPER_GUIDE.md) - How to develop plugins
- [Plugin System Architecture](PLUGIN_SYSTEM.md) - Overall plugin system design
