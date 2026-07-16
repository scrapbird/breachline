---
title: "Timestamps & Timezones"
date: 2025-01-01T00:00:00Z
draft: false
weight: 2
---

BreachLine is built around time. Every row is treated as a timestamped event, so getting the timestamp column and timezones right is the most important part of loading a dataset correctly.

## Timestamp detection

When you open a file, BreachLine picks a timestamp column automatically:

1. **Exact name match** first: a column named `@timestamp`, `timestamp`, or `time`.
2. **Partial name match** next: a column whose name contains `@timestamp`, `timestamp`, `datetime`, `date`, `time`, or `ts`.
3. **Fallback**: the first column, if nothing else matches.

If BreachLine guesses wrong, click the column header and choose **Use as timestamp**. The grid re-sorts and the histogram redraws against the column you picked.

## Supported timestamp formats

BreachLine parses a wide range of formats without configuration:

- **ISO 8601** with or without a timezone, for example `2025-08-01T15:04:05Z` or `2025-08-01T15:04:05`.
- **Space-separated** date and time, for example `2025-08-01 15:04:05`.
- **Fractional seconds** at several precisions, for example `.0`, `.00`, `.000`.
- **Numeric offsets and named zones**, for example `+13:00` or `UTC`.
- **Epoch** values as integer seconds or milliseconds.

Rows whose timestamp cannot be parsed are kept and sorted together, ordered after the rows that do parse, so nothing is silently dropped.

## Ingest timezone

Some timestamps carry no timezone of their own, for example `2025-08-01 15:04:05`. To turn those into an exact point in time, BreachLine needs to know which zone they were recorded in. That is the **ingest timezone**.

- It is applied only to timestamps that do **not** already specify an offset.
- Timestamps that already carry a zone (a `Z` suffix or a numeric offset) are respected as-is and are unaffected by this setting.
- The default is your machine's local timezone. Set it explicitly under **File → Settings** when your logs were recorded somewhere else, for example a server running in UTC.

Getting this wrong shifts every zone-less timestamp by the offset between the two zones, so set it to match the source of your data.

## Display timezone

The **display timezone** controls how timestamps are shown in the grid, the histogram axis, and exports. It is separate from the ingest timezone.

- Changing it re-renders the existing data. It does not re-read or re-parse the file, so it is fast.
- A common workflow is to ingest in the zone the logs were written in, then set the display timezone to UTC so timestamps line up across sources.

## Display format

The **timestamp display format** controls the exact text used to render each timestamp, for example `yyyy-MM-dd HH:mm:ss`. Set it under **File → Settings**. It affects the grid and any exported timeline.

## Where these settings live

The ingest timezone, display timezone, and display format are all part of the application settings. See the [config system](/docs/configuration/config-system/) for where they are stored and how defaults work.
