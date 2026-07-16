---
title: "Local Workspaces"
date: 2025-01-01T00:00:00Z
draft: false
weight: 2
---

A local workspace is a single `.breachline` file on your disk. It records which files belong to the investigation and every annotation you have made, with no cloud component.

## Creating a workspace

1. Choose **Workspace → Open Workspace** and pick a location and name for the new `.breachline` file, or open an existing one.
2. Add files with **Workspace → Add Files**. Each file is recorded by path and by a hash of its contents.
3. Annotate rows as you investigate. See [Annotations](/docs/workspaces/annotations/).

The workspace file is saved as you go, so your annotations are never lost.

## What is stored

The `.breachline` file records, per file in the workspace:

- The file path and a content hash.
- For JSON files, the JPATH expression used to extract records, so the same file can appear more than once with different expressions.
- An optional description.
- The annotations attached to that file's rows.

The workspace stores references to your data files, not the data itself. The original CSV, XLSX, or JSON files stay where they are on disk.

## Sharing a workspace

Because a workspace is a single file, you can share it with a colleague. For it to open cleanly on their machine, they need access to the same underlying data files at the recorded paths. To share a self-contained result instead, use [Exporting a Timeline](/docs/workspaces/exporting-a-timeline/), which merges the annotated data into one file.

## Keeping it in sync

To keep a workspace up to date across more than one machine automatically, use a [sync workspace](/docs/workspaces/sync-workspaces/) instead.
