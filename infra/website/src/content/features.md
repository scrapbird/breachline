---
title: "Features"
date: 2025-01-01T00:00:00Z
draft: false
---

# Features

## Large File Handling

Load and analyze massive datasets up to 5-10 GB in size. Each row represents a timestamped event, making BreachLine perfect for processing extensive audit logs and security events during incident response investigations.

## Native Format Support

- **CSV Files**: Standard comma-separated values
- **XLSX Files**: Excel spreadsheets
- **JSON Files**: With custom JPATH expressions to locate and ingest list data

Flexibly import data from various sources and formats used in security operations.

## Ingest Plugins

BreachLine supports plugins to ingest data from various sources and formats used in security operations. Write custom plugins in any language to ingest data from any source or file type.

## Advanced Query Language

Powerful SPL-like (Splunk Processing Language) filter syntax for real-time data filtering. Create complex queries to quickly isolate relevant events and identify patterns in your timeline data.

## High-Performance Architecture

- **Sort and Query Cache**: Fast re-sorting and re-querying without reprocessing
- **External Sort**: Handle files larger than available memory using temporary file sorting
- **Virtualized Data Grid**: Responsive interface that only renders visible rows for optimal performance

## Timeline Visualization

- **Interactive Histograms**: View event counts across time buckets (1 hour, 5 minutes, etc.)
- **Time Navigation**: Seek by time to quickly jump to specific periods
- **Event Graphs**: Visual representation of activity patterns

## Flexible Timezone Handling

- Configure default ingest timezone for imported data
- Set separate display timezone for analysis
- Normalize timestamps to a configurable time format

Perfect for investigations spanning multiple geographic regions.

## Workspace & Annotations

- Save workspaces with annotations and notes
- Collaborate on investigations by sharing annotated timelines
- Export annotated data to combined timeline files

## Cross-Platform Support

BreachLine works seamlessly across:
- Windows 10/11
- macOS 12+
- Linux (Ubuntu, Fedora, Arch, and other distributions)

## Built for Incident Response

Designed specifically for cybersecurity professionals conducting forensic analysis and incident investigations. Fast, efficient, and purpose-built for the challenges of timeline analysis.
