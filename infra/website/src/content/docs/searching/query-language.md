---
title: "Query Language"
date: 2025-01-01T00:00:00Z
draft: false
weight: 1
---

BreachLine's query language is an SPL-like syntax designed for fast filtering of timestamped records.

## Field comparisons

Compare a field to a value with the standard operators:

| Operator | Meaning                  | Example                    |
|----------|--------------------------|----------------------------|
| `=`      | equals                   | `status="FAILED"`          |
| `!=`     | not equals               | `user!="root"`             |
| `>` `<`  | greater / less than      | `bytes>1048576`            |
| `>=` `<=`| greater / less or equal  | `status_code>=400`         |
| `~`      | matches (substring/regex)| `path~"/admin"`            |

## Boolean logic

Combine terms with `AND`, `OR`, and `NOT`. Use parentheses to group:

```
(status_code>=500 OR status_code=403) AND NOT source_ip="10.0.0.1"
```

A bare term with no field is treated as a full-text match across all columns:

```
"powershell" AND event_id=4688
```

## Time filters

Restrict results to a window with `before` and `after`:

```
after "2025-08-01T00:00:00" before "2025-08-02T00:00:00"
```

Relative ranges are also supported:

```
after -24h
```

## Dedup

Collapse duplicate rows on one or more fields with `dedup`:

```
dedup source_ip, user_agent
```

## Putting it together

```
status_code>=400 AND path~"/api/" after -7d dedup source_ip
```

This finds error responses on API routes in the last seven days, keeping one row per source IP.

See the [Query Pipeline](/docs/searching/query-pipeline/) guide for column projection and more detail on time filters.
