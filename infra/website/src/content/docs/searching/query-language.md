---
title: "Query Language"
date: 2025-01-01T00:00:00Z
draft: false
weight: 1
---

BreachLine's query language is an SPL-like syntax designed for fast filtering of timestamped records.

A query is a set of stages joined by the pipe character (`|`), and every stage starts with a keyword. To match rows, use the `filter` stage:

```
filter status="FAILED"
```

The rest of this page covers what goes inside a `filter` stage. See the [Query Pipeline](/docs/searching/query-pipeline/) guide for the other stages (`columns`, `after`, `before`, `dedup`, `limit`) and how to chain them.

## Field comparisons

Compare a field to a value with these operators:

| Operator | Meaning                               | Example                    |
|----------|---------------------------------------|----------------------------|
| `=`      | equals (case-insensitive)             | `status="FAILED"`          |
| `!=`     | not equals                            | `user!="root"`             |
| `~`      | contains (case-insensitive substring) | `path~"/admin"`            |
| `!~`     | does not contain                      | `path!~"/health"`          |

Two shortcuts use a trailing `*`:

- `field=*` matches any row where the field has a value. For example `filter errorCode=*` keeps only rows that carry an error code.
- `field=value*` matches on a prefix. For example `filter eventName=Delete*` matches `DeleteRole`, `DeleteBucket`, and so on.

Values that contain spaces must be quoted, for both the field and the value: `filter "user name"="Jane Doe"`.

## Boolean logic

Combine terms with `AND`, `OR`, and `NOT`. A space between terms is treated as `AND`. Use parentheses to group:

```
filter (status="FAILED" OR responseCode=403) AND NOT sourceIPAddress="10.0.0.1"
```

A bare term with no field is treated as a full-text substring match across all columns:

```
filter "powershell" AND eventName=RunInstances
```

## Time filters

`after` and `before` are their own stages. They accept absolute and relative values, and you can use both in one stage to bound a window:

```
after "2025-08-01T00:00:00" before "2025-08-02T00:00:00"
```

Relative values are an offset back from now, written as a number and a unit (`s`, `m`, `h`, `d`, `w`, `mo`, `y`). `after 24h` means the last 24 hours:

```
after 24h
```

## Dedup

`dedup` is its own stage. It keeps one row per unique combination of the fields you name:

```
dedup sourceIPAddress, userAgent
```

## Putting it together

Stages compose left to right, each separated by a pipe:

```
filter "powershell" AND eventName=RunInstances | after 7d | dedup sourceIPAddress
```

This finds full-text matches for PowerShell on `RunInstances` events in the last seven days, and keeps one row per source IP.
