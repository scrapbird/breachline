---
title: "Cache System"
date: 2025-01-01T00:00:00Z
draft: false
weight: 3
---

BreachLine stays fast on large files by caching work it has already done. There are two distinct caches: a per-tab sort cache and a shared query cache.

## Sort cache

The first time a query needs sorted data, BreachLine reads the whole file into memory and sorts it by the timestamp column. That sorted copy is cached and reused, so later queries against the same file do not pay the sort cost again.

The sort cache is held per tab, so each open file has its own. It is rebuilt when any of these change:

- The file itself.
- Whether sorting by time is enabled.
- The sort direction (ascending or descending).
- The timestamp column in use.

## Query cache

On top of the sort cache, BreachLine caches query results so that re-running a recent query, or paging back to it, returns instantly instead of re-filtering. It caches at two levels: whole-query results and the results of individual pipeline stages, so even a query that is only partly the same as a previous one can reuse work.

Unlike the sort cache, the query cache is a single cache shared across every open tab, not one cache per tab. Entries are keyed by file and by the settings that affect results, so tabs never serve each other the wrong rows; they simply share one pool of cached results.

The pool is bounded by a memory budget rather than by a number of queries. When it is full, the least recently used entries are evicted first. The budget is configurable (see the query cache size setting in the [config system](/docs/configuration/config-system/); it defaults to 100 MB), and you can turn the query cache off entirely under **File → Settings**. Sorting is unaffected by that toggle.

## When caches are cleared

Caches are invalidated automatically when a change would make them stale, for example changing the sort order, the timestamp column, or a timezone or format setting. Query-cache entries for a file are dropped when that file's timestamp column changes, and closing a tab frees the sort cache that tab held.

## Memory trade-off

Caching trades memory for speed. On a machine that is tight on RAM relative to the file size, lower the query cache size or turn the query cache off under **File → Settings** to reduce memory use at the cost of re-running filters. See [System Requirements](/docs/getting-started/system-requirements/) for sizing guidance.
