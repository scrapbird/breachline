---
title: "Loading Data"
date: 2025-01-01T00:00:00Z
draft: false
weight: 1
---

BreachLine reads several common formats and can treat a whole directory as one logical dataset.

## Supported formats

- **CSV** - headers are detected automatically; delimiter is inferred.
- **XLSX** - the first worksheet is loaded by default.
- **JSON** - arrays of objects, or newline-delimited JSON (NDJSON).

Need a format that is not listed here? See the [plugin system](/docs/extending/plugin-system/), which lets a loader plugin add support for any format.

## Opening a directory

Choose **File → Open Directory** to load every supported file in a folder as a single dataset. This is ideal for exports that are split into many files, such as CloudTrail logs:

```
cloudtrail/
  2025/08/01/000000.json
  2025/08/01/010000.json
  ...
```

All records are merged and sorted by timestamp.

## Compressed files

BreachLine decompresses archived files automatically when you open them. A gzip file (`.gz`) is unzipped in memory on open and read as whatever format it contains, so opening `cloudtrail.json.gz` behaves exactly like opening the `cloudtrail.json` inside it, with no need to extract it first. bzip2 (`.bz2`) and xz (`.xz`) archives are handled the same way. Compression is detected both from the file extension and from the file's contents, so a compressed file with an unusual name is still recognised.

This pairs naturally with opening a directory. Point BreachLine at a folder of compressed, partitioned exports and it decompresses and merges every file into one time-ordered dataset. That is the usual shape of a CloudTrail archive, whose events arrive as many small gzip files split across date-partitioned folders:

```
cloudtrail/
  2025/08/01/000000.json.gz
  2025/08/01/010000.json.gz
  ...
```

Open the top-level folder and the whole archive becomes a single searchable timeline.

## Custom JPATH expressions

For nested JSON, use a JPATH expression to tell BreachLine where the records live. Given a file shaped like this:

```json
{
  "Records": [
    { "eventTime": "2025-08-01T12:00:00Z", "eventName": "ConsoleLogin" }
  ]
}
```

set the records path to:

```
$.Records
```

Each element of `Records` becomes a row, and its keys become columns. Nested objects and arrays inside a record are kept as JSON text in the cell.

## Large files

Large files are loaded fully into memory and backed by intelligent caching, so scrolling and querying stay fast once a dataset is open. See [System Requirements](/docs/getting-started/system-requirements/) for guidance on file size, and the [Query Language](/docs/searching/query-language/) guide to start filtering.
