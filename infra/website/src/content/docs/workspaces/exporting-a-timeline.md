---
title: "Exporting a Timeline"
date: 2025-01-01T00:00:00Z
draft: false
weight: 6
---

Exporting a timeline merges the files in a workspace into one time-ordered file, carrying your annotations with it. It turns an investigation spread across many logs into a single deliverable.

Exporting is a [licensed feature](/docs/getting-started/licensing/) and needs an open workspace.

## Exporting

1. Open the [workspace](/docs/workspaces/overview/) you want to export.
2. Choose **Workspace → Export Timeline**.
3. Pick a destination file.

BreachLine reads every file in the workspace, merges their rows, orders them by timestamp, and writes a single combined timeline. Annotations are included, so the notes and colors you added travel with the exported file.

## What you get

- One file containing rows from every file in the workspace.
- Rows sorted by timestamp across all sources, so activity from different logs interleaves in true chronological order.
- Your annotations attached to the rows they mark.

Timestamps are written using your [display timezone and display format](/docs/loading-data/timestamps-timezones/).

## When to use it

An exported timeline is the right way to hand off results, because it is self-contained: unlike a `.breachline` [workspace file](/docs/workspaces/local-workspaces/), it embeds the data rather than referencing files on disk. Share it with someone who does not have the original logs, or attach it to a report.
