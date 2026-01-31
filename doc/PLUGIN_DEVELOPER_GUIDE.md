# BreachLine Plugin Developer Guide

## Overview

BreachLine plugins allow you to load custom file formats by implementing an executable that converts your file format to CSV. Plugins can be written in any language (Python, Go, Rust, shell scripts, etc.) as long as they follow the plugin API specification.

## Quick Start

1. **Create a plugin directory**:
   ```
   my-plugin/
     plugin.yml
     my-loader
   ```

2. **Write a `plugin.yml` manifest**:
   ```yaml
   id: 12345678-1234-1234-1234-123456789abc  # Generate a unique UUID
   name: My Custom Loader
   version: 1.0.0
   description: Loads my custom file format
   executable: my-loader
   extensions:
     - .custom
   author: Your Name
   ```
   
   Generate a unique UUID for your plugin using:
   - Linux/macOS: `uuidgen`
   - PowerShell: `[guid]::NewGuid()`
   - Python: `python3 -c "import uuid; print(uuid.uuid4())"`

3. **Implement the loader executable** that supports three modes:
   - `--mode=header`: Output CSV header row
   - `--mode=count`: Output row count
   - `--mode=stream`: Output full CSV

4. **Make it executable** (Unix-like systems):
   ```bash
   chmod +x my-loader
   ```

5. **Add it to BreachLine** via Settings → Plugins → Add Plugin

## Plugin API Specification

### Command-Line Interface

Your plugin executable must accept two command-line arguments:

```bash
<plugin-executable> --mode=<mode> --file=<filepath>
```

**Arguments**:
- `--mode`: One of `header`, `count`, or `stream`
- `--file`: Absolute path to the file to process

### Execution Modes

#### Header Mode

**Command**: `--mode=header`

**Purpose**: Return the CSV header row

**Output**: Single line of CSV containing column names

**Example**:
```csv
timestamp,user_id,action,details
```

**Requirements**:
- Output must be valid CSV (use proper escaping)
- Output exactly one line (the header)
- Exit with code 0 on success

#### Count Mode

**Command**: `--mode=count`

**Purpose**: Return the number of data rows (excluding header)

**Output**: Single integer on stdout

**Example**:
```
1523
```

**Requirements**:
- Output a single non-negative integer
- Do NOT include the header in the count
- Exit with code 0 on success

#### Stream Mode

**Command**: `--mode=stream`

**Purpose**: Return the full CSV output (header + all data rows)

**Output**: Complete CSV file with header and all rows

**Example**:
```csv
timestamp,user_id,action,details
2024-01-15T10:30:00Z,user123,login,Success
2024-01-15T10:31:42Z,user456,view_page,/products
2024-01-15T10:32:18Z,user123,logout,Session ended
```

**Requirements**:
- First line must be the header
- All subsequent lines are data rows
- Output must be valid CSV
- Exit with code 0 on success

### Error Handling

**Exit Codes**:
- `0`: Success
- Non-zero: Error

**Error Reporting**:
- Write error messages to **stderr** (not stdout)
- Include descriptive error messages
- BreachLine will show stderr output to the user

**Example Error Handling** (Python):
```python
try:
    # Process file
    pass
except FileNotFoundError:
    print(f"File not found: {file_path}", file=sys.stderr)
    sys.exit(1)
except Exception as e:
    print(f"Error: {e}", file=sys.stderr)
    sys.exit(1)
```

### CSV Output Format

**Requirements**:
- Use standard CSV format (RFC 4180)
- Use comma (`,`) as delimiter
- Quote fields containing commas, newlines, or quotes
- Escape quotes within quoted fields by doubling them (`""`)

**Use CSV libraries** to ensure proper formatting:
- Python: `csv` module
- Go: `encoding/csv` package
- Rust: `csv` crate

## Implementation Examples

### Python Example

```python
#!/usr/bin/env python3
import argparse
import csv
import sys

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--mode', required=True, choices=['header', 'count', 'stream'])
    parser.add_argument('--file', required=True)
    args = parser.parse_args()
    
    try:
        if args.mode == 'header':
            writer = csv.writer(sys.stdout)
            writer.writerow(['column1', 'column2', 'column3'])
        
        elif args.mode == 'count':
            count = count_rows(args.file)
            print(count)
        
        elif args.mode == 'stream':
            writer = csv.writer(sys.stdout)
            writer.writerow(['column1', 'column2', 'column3'])
            for row in read_custom_format(args.file):
                writer.writerow(row)
    
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()
```

### Go Example

```go
package main

import (
    "encoding/csv"
    "flag"
    "fmt"
    "os"
)

func main() {
    mode := flag.String("mode", "", "Execution mode")
    file := flag.String("file", "", "File path")
    flag.Parse()

    switch *mode {
    case "header":
        outputHeader()
    case "count":
        count, err := countRows(*file)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Println(count)
    case "stream":
        if err := outputStream(*file); err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
    default:
        fmt.Fprintf(os.Stderr, "Invalid mode: %s\n", *mode)
        os.Exit(1)
    }
}

func outputHeader() {
    w := csv.NewWriter(os.Stdout)
    w.Write([]string{"column1", "column2", "column3"})
    w.Flush()
}

func outputStream(filePath string) error {
    w := csv.NewWriter(os.Stdout)
    defer w.Flush()
    
    // Write header
    w.Write([]string{"column1", "column2", "column3"})
    
    // Read custom format and write rows
    rows, err := readCustomFormat(filePath)
    if err != nil {
        return err
    }
    
    for _, row := range rows {
        w.Write(row)
    }
    
    return nil
}
```

### Shell Script Example

```bash
#!/bin/bash

set -e  # Exit on error

MODE=""
FILE=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --mode=*)
            MODE="${1#*=}"
            shift
            ;;
        --file=*)
            FILE="${1#*=}"
            shift
            ;;
        *)
            echo "Unknown argument: $1" >&2
            exit 1
            ;;
    esac
done

case "$MODE" in
    header)
        echo "column1,column2,column3"
        ;;
    count)
        wc -l < "$FILE" | tr -d ' '
        ;;
    stream)
        echo "column1,column2,column3"
        # Process file and output CSV rows
        while IFS= read -r line; do
            # Transform line to CSV format
            echo "$line" | awk -F'|' '{print $1 "," $2 "," $3}'
        done < "$FILE"
        ;;
    *)
        echo "Invalid mode: $MODE" >&2
        exit 1
        ;;
esac
```

## Testing Your Plugin

### Standalone Testing

Test your plugin before adding it to BreachLine:

```bash
# Test header mode
./my-loader --mode=header --file=/path/to/test.custom

# Test count mode
./my-loader --mode=count --file=/path/to/test.custom

# Test stream mode
./my-loader --mode=stream --file=/path/to/test.custom | head -20
```

### Validation Checklist

- [ ] All three modes work correctly
- [ ] Header mode outputs valid CSV header
- [ ] Count mode outputs correct row count (excluding header)
- [ ] Stream mode outputs valid CSV with header + rows
- [ ] Errors are written to stderr
- [ ] Non-zero exit code on error
- [ ] Handles missing files gracefully
- [ ] Handles large files efficiently
- [ ] CSV output is properly escaped

## Best Practices

### Performance

1. **Stream processing**: Process files line-by-line or in chunks to handle large files
2. **Avoid loading entire file into memory**: Especially for `stream` mode
3. **Efficient counting**: Count rows without loading full content if possible

### Error Handling

1. **Be specific**: Provide clear error messages
2. **Validate input**: Check if file exists and is readable
3. **Handle encoding**: Use UTF-8 or handle encoding errors gracefully
4. **Fail fast**: Exit immediately on unrecoverable errors

### CSV Generation

1. **Use CSV libraries**: Don't hand-craft CSV output
2. **Escape properly**: Commas, quotes, and newlines must be escaped
3. **Consistent headers**: Same header across all modes
4. **Column count**: Ensure all rows have the same number of columns

### Plugin Manifest

1. **Unique ID**: Always generate a unique UUID for your plugin
2. **Never change the ID**: Once published, the ID should never change (it's used for cache keys and annotations)
3. **Use semver**: Version your plugin with semantic versioning
4. **Clear descriptions**: Help users understand what your plugin does
5. **Specific extensions**: Only register extensions you actually support
6. **Test manifest**: Ensure `plugin.yml` is valid YAML

## Security Considerations

**Plugins run with full user permissions**. Be aware:

- Plugins can access any file accessible to BreachLine
- Plugins can execute arbitrary code
- No sandboxing is currently implemented
- Users should only install plugins from trusted sources

**As a plugin developer**:
- Validate all inputs
- Don't execute shell commands with user input
- Handle file paths safely (avoid path traversal)
- Document any external dependencies
- Be transparent about what your plugin does

## Troubleshooting

### Plugin Not Loading

**Problem**: Plugin doesn't appear in BreachLine

**Solutions**:
- Check `plugin.yml` is in the same directory as executable
- Ensure `plugin.yml` is valid YAML
- Verify executable has execute permissions (`chmod +x`)
- Check BreachLine logs for error messages

### Invalid CSV Output

**Problem**: BreachLine shows CSV parsing errors

**Solutions**:
- Use a CSV library instead of string concatenation
- Verify all rows have the same number of columns
- Check for unescaped special characters (commas, quotes, newlines)
- Test with a CSV validator

### Plugin Fails on Large Files

**Problem**: Plugin crashes or times out on large files

**Solutions**:
- Use streaming/iterative processing
- Don't load entire file into memory
- Process in chunks
- Optimize count mode to avoid reading full content

### Wrong Row Count

**Problem**: Count mode returns incorrect count

**Solutions**:
- Don't include the header in the count
- Count data rows only
- Handle empty files correctly (return 0)

## Advanced Topics

### Binary File Formats

For binary formats (Parquet, Protocol Buffers, etc.):
- Use appropriate libraries for parsing
- Convert to CSV for BreachLine consumption
- Handle endianness and encoding correctly

### Compressed Files

BreachLine handles compression (.gz, .bz2, .xz) automatically. If your format has built-in compression:
- Decompress within your plugin
- Output uncompressed CSV

### Configuration

Future versions may support plugin-specific configuration. For now:
- Use environment variables for configuration
- Document configuration requirements
- Provide sensible defaults

### Timestamps

If your format has timestamps:
- Output them in ISO 8601 format when possible
- Use consistent timezone (prefer UTC)
- Include timezone information in the timestamp string

## Resources

- [Plugin Manifest Specification](PLUGIN_MANIFEST_SPEC.md)
- [Plugin System Architecture](PLUGIN_SYSTEM.md)
- [Example Plugin](../tools/example-plugin/)

## Getting Help

- Check the BreachLine logs for error messages
- Test your plugin standalone before adding to BreachLine
- Verify your `plugin.yml` against the specification
- Review the example plugin for reference implementation

## License

Plugins are independent software and can use any license. Clearly document your plugin's license in its documentation.
