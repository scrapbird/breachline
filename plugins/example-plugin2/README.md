# Example BreachLine Plugin

This is an example plugin that demonstrates how to create a BreachLine file loader plugin.

## What It Does

This plugin reads any text file and converts it to CSV format with two columns:
- `line_number`: The line number (1-indexed)
- `content`: The line content converted to uppercase

## Installation

1. Make the plugin executable (on Unix-like systems):
   ```bash
   chmod +x example-loader
   ```

2. Add the plugin to BreachLine:
   - Open BreachLine settings
   - Navigate to the Plugins tab
   - Click "Add Plugin"
   - Select the `example-loader` executable (or the `example-plugin` directory)

3. Create a test file:
   ```bash
   echo -e "hello world\nthis is a test\nbreachline plugins" > test.example
   ```

4. Open `test.example` in BreachLine to see the plugin in action

## Plugin Structure

```
example-plugin/
  plugin.yml          # Plugin manifest
  example-loader      # Plugin executable (Python script)
  README.md          # This file
```

## Plugin API

The plugin implements three modes as required by the BreachLine plugin API:

### Header Mode
```bash
./example-loader --mode=header --file=/path/to/file.example
```
Output: CSV header row

### Count Mode
```bash
./example-loader --mode=count --file=/path/to/file.example
```
Output: Number of rows (excluding header)

### Stream Mode
```bash
./example-loader --mode=stream --file=/path/to/file.example
```
Output: Full CSV (header + data rows)

## Testing the Plugin Standalone

You can test the plugin without BreachLine:

```bash
# Test header mode
./example-loader --mode=header --file=test.example

# Test count mode
./example-loader --mode=count --file=test.example

# Test stream mode
./example-loader --mode=stream --file=test.example
```

## Modifying the Plugin

This is a simple example. You can modify it to:
- Support different file formats (Parquet, Protocol Buffers, etc.)
- Parse complex binary formats
- Extract data from proprietary formats
- Transform data during loading

See the [Plugin Developer Guide](../../doc/PLUGIN_DEVELOPER_GUIDE.md) for more information.

## Requirements

- Python 3.6 or higher (only required if you run the plugin standalone)
- BreachLine with plugin support enabled

## License

This example plugin is provided as-is for demonstration purposes.
