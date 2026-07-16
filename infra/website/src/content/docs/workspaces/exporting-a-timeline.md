---
title: "Exporting a Timeline"
date: 2025-01-01T00:00:00Z
draft: false
weight: 6
---

Exporting a timeline collects the rows you have annotated across a workspace into one time-ordered CSV. It turns the findings you marked during an investigation, spread across many logs, into a single deliverable.

Exporting is a [licensed feature](/docs/getting-started/licensing/) and needs an open workspace.

## Exporting

1. Open the [workspace](/docs/workspaces/overview/) you want to export.
2. Choose **Workspace → Export Timeline**.
3. Pick a destination for the CSV file.

BreachLine reads each file that has annotations, picks out only the annotated rows, merges them, orders them by timestamp, and writes a single combined CSV. Files with no annotations are skipped, and if nothing in the workspace is annotated the export stops and tells you there is nothing to export.

## What you get

- A CSV containing only your annotated rows, gathered from every annotated file in the workspace. Rows you did not annotate are not included.
- Rows sorted by timestamp across all sources, so the findings from different logs interleave in true chronological order.
- Extra leading columns on each row that record where it came from and what you noted: the timestamp, the source file path, that file's description, and your annotation note. The row's own original columns follow.

Timestamps are written using your [display timezone and display format](/docs/loading-data/timestamps-timezones/).

## When to use it

An exported timeline is the right way to hand off results, because it is a focused, self-contained record of your findings. Unlike a `.breachline` [workspace file](/docs/workspaces/local-workspaces/), it embeds the annotated rows themselves rather than referencing files on disk, so you can share it with someone who does not have the original logs, or attach it to a report. If instead you need every row of the source data, open the files directly and use [Copying & Exporting](/docs/searching/copying-exporting/).
