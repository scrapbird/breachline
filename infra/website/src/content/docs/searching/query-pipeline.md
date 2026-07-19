---
title: "Query Pipeline"
date: 2025-01-01T00:00:00Z
draft: false
weight: 2
---

A query is a set of stages joined by the pipe character (`|`). Every stage starts with a keyword: `filter` to match rows, then `columns`, `after`, `before`, `dedup`, and `limit` to shape the result. This page covers the stages beyond basic field matching. For the `filter` grammar itself, see the [Query Language](/docs/searching/query-language/) guide.

## Column projection

Reduce a wide dataset to just the columns you care about with `columns`:

```
filter status="FAILED" | columns timestamp, source_ip, path
```

- Column names are matched case-insensitively.
- Projection affects the grid, copied selections, and exports, so it is a fast way to produce a focused timeline.
- The timestamp column is always available for sorting even if you do not list it.

## Time filters

`after` and `before` are their own stages that restrict results to a time window. Both accept absolute and relative values, and you can combine them in one stage.

**Absolute** times are interpreted in your [display timezone](/docs/loading-data/timestamps-timezones/):

```
after "2025-08-01T00:00:00" before "2025-08-02T00:00:00"
```

**Relative** times are an offset back from now, written as a number and a unit:

```
after 24h
```

Supported units:

| Unit | Meaning |
|------|---------|
| `s`  | seconds |
| `m`  | minutes |
| `h`  | hours   |
| `d`  | days    |
| `w`  | weeks   |
| `mo` | months  |
| `y`  | years   |

You can also select a range directly on the [timeline histogram](/docs/searching/histogram/), which writes the matching `after`/`before` stage into your query for you.

## Dedup

Keep one row per unique combination of fields with `dedup`:

```
dedup source_ip, user_agent
```

## Limit

Cap the number of matched rows with `limit`:

```
filter status="FAILED" | limit 100
```

## Combining stages

Stages compose left to right:

```
filter "powershell" AND event_id=4688 | after 7d | columns timestamp, host, user | dedup user
```

This finds full-text matches for PowerShell on process-creation events in the last seven days, narrows to three columns, and keeps one row per user.
