---
title: "Annotations"
date: 2025-01-01T00:00:00Z
draft: false
weight: 5
---

Annotations are notes you attach to specific rows during an investigation. They carry a note and a color, travel with the workspace, and survive re-sorting and re-querying.

Annotations are a [licensed feature](/docs/getting-started/licensing/) and require an open workspace.

## Adding an annotation

1. Open a [workspace](/docs/workspaces/overview/) and make sure the file is part of it.
2. Select one or more rows in the grid.
3. Right-click and choose **Add annotation**.
4. Enter a note, pick a color, and save.

Annotated rows are highlighted in the grid in the color you chose. To edit or remove an annotation, right-click the row again.

## Colors

Annotations support six colors: grey, blue, yellow, green, orange, and red. Use them however suits your workflow, for example red for confirmed malicious activity and yellow for items still under review.

## How rows are matched

An annotation is tied to the content of a row, not its position. BreachLine hashes the row's column values and stores that with the annotation. This means:

- Sorting or filtering the data does not detach an annotation from its row.
- If the underlying file changes so that a row's values differ, its annotation no longer matches that row.

Because matching is content-based, the same annotation lines up correctly even when the file is opened from a different path on another machine.

## Exporting annotations

To produce a single file containing every annotated row across the workspace, see [Exporting a Timeline](/docs/workspaces/exporting-a-timeline/).
