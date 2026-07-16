---
title: "System Requirements"
date: 2025-01-01T00:00:00Z
draft: false
weight: 4
---

BreachLine is a single self-contained desktop application. There is no runtime to install, no database to run, and no background service.

## Supported platforms

- **Windows** 10 and later (64-bit).
- **macOS** 12 (Monterey) and later, Apple Silicon and Intel.
- **Linux** modern 64-bit distributions, distributed as a native binary in a `.tar.gz` archive.

See [Installation](/docs/getting-started/installation/) for per-platform steps.

## Memory and file size

BreachLine loads a file fully into memory and keeps sorted and filtered results cached for speed. That makes scrolling and re-querying fast, but it means the practical file-size ceiling is governed by available RAM.

- Files up to roughly **5-10 GB** are supported on a well-resourced machine.
- As a rule of thumb, keep a comfortable margin of free RAM above the size of the file you are opening, because the sort and query caches use additional memory on top of the raw data.
- Opening a directory merges every file in it into one dataset, so the combined size is what matters, not the size of any single file.

If you are working near the ceiling, the [cache system](/docs/configuration/cache-system/) page explains how caching uses memory and how to reduce its footprint.

## Disk and network

- BreachLine reads your data files from local disk. It does not copy them.
- No network connection is required to open files or run queries.
- Sync and license activation are the only features that use the network. See [Security & data handling](/docs/reference/security-data-handling/) for exactly what is transmitted.
