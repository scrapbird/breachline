---
title: "Plugin Developer Guide"
date: 2025-01-01T00:00:00Z
draft: false
weight: 2
---

A BreachLine plugin is an executable that converts a file format into CSV. Because BreachLine talks to it over a small command-line contract, you can write a plugin in any language: Python, Go, Rust, a shell script, anything that reads a file and writes CSV.

## Anatomy of a plugin

A plugin is a directory containing a manifest and an executable:

```
my-plugin/
  plugin.yml     # manifest (required)
  my-loader      # executable (required)
```

### The manifest

`plugin.yml` describes the plugin:

```yaml
id: 12345678-1234-1234-1234-123456789abc   # a unique UUID, generated once
name: My Custom Loader
version: 1.0.0
description: Loads my custom file format
executable: my-loader
extensions:
  - .custom
author: Your Name
```

Required fields are `id`, `name`, `version`, `executable`, and `extensions`; `description` and `author` are optional. Generate the `id` once with `uuidgen`, `[guid]::NewGuid()`, or `python3 -c "import uuid; print(uuid.uuid4())"`, and never change it: BreachLine uses it as a stable key for caches and annotations. Each extension must start with a dot and is matched case-insensitively.

## The command-line contract

BreachLine invokes your executable with two arguments:

```bash
my-loader --mode=<mode> --file=<absolute-path>
```

Your plugin must support three modes.

### header

Print the CSV header row (one line) and exit `0`.

```csv
timestamp,user_id,action,details
```

### count

Print the number of data rows, excluding the header, as a single integer, and exit `0`.

```
1523
```

### stream

Print the full CSV: the header followed by every data row, and exit `0`.

```csv
timestamp,user_id,action,details
2025-01-15T10:30:00Z,user123,login,Success
2025-01-15T10:31:42Z,user456,view_page,/products
```

## Rules to follow

- **Write CSV with a library**, not by hand, so commas, quotes, and newlines are escaped correctly (RFC 4180). Use Python's `csv`, Go's `encoding/csv`, or the Rust `csv` crate.
- **Keep the header identical** across all three modes, with the same column count on every row.
- **Report errors on stderr** and exit non-zero. BreachLine shows stderr to the user.
- **Stream, do not slurp.** Process large files line by line rather than loading them entirely into memory, especially in `stream` mode.
- **Emit ISO 8601 timestamps in UTC** where you can, so BreachLine's timestamp detection works cleanly. See [Timestamps & Timezones](/docs/loading-data/timestamps-timezones/).

## A minimal Python plugin

```python
#!/usr/bin/env python3
import argparse
import csv
import sys


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--mode', required=True,
                        choices=['header', 'count', 'stream'])
    parser.add_argument('--file', required=True)
    args = parser.parse_args()

    header = ['timestamp', 'user_id', 'action', 'details']

    try:
        if args.mode == 'header':
            csv.writer(sys.stdout).writerow(header)
        elif args.mode == 'count':
            print(count_rows(args.file))
        elif args.mode == 'stream':
            writer = csv.writer(sys.stdout)
            writer.writerow(header)
            for row in read_rows(args.file):
                writer.writerow(row)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
```

## Test before you install

Run each mode from a terminal before adding the plugin to BreachLine:

```bash
./my-loader --mode=header --file=/path/to/test.custom
./my-loader --mode=count  --file=/path/to/test.custom
./my-loader --mode=stream --file=/path/to/test.custom | head -20
```

Check that header and stream agree on columns, that count excludes the header, that errors go to stderr with a non-zero exit code, and that a missing file is handled gracefully.

## Installing it

Make the executable runnable (`chmod +x my-loader` on macOS and Linux) and add it through **File → Settings → Plugins → Add Plugin**. See the [Plugin System](/docs/extending/plugin-system/) page for the user-facing side and the trust considerations that apply to anyone installing your plugin.

## Compression

BreachLine handles `.gz`, `.bz2`, and `.xz` files automatically. If your own format is internally compressed, decompress inside the plugin and output plain CSV.
