---
title: "Copying & Exporting"
date: 2025-01-01T00:00:00Z
draft: false
weight: 5
---

Once a query has narrowed the data to what matters, you can pull the results out of BreachLine to paste into a report, a spreadsheet, or a ticket.

## Copy selected rows

1. Select rows in the grid. Click to select one, shift-click to select a range, and use **Ctrl/Cmd+A** to select every row matching the current query.
2. Copy with **Ctrl/Cmd+C** or **File → Copy**.

Rows are copied as tab-separated values (TSV), so they paste cleanly into Excel, Google Sheets, or any spreadsheet. If your query includes a [`columns` projection](/docs/searching/query-pipeline/), only those columns are copied.

Select-all works on the whole filtered result set, not just the rows currently scrolled into view, so you can copy millions of matching rows without scrolling through them.

## Copy the histogram

Copy the [timeline histogram](/docs/searching/histogram/) as a PNG image to paste a visual of event volume straight into a report.

## Export a full timeline

To export a merged, annotated timeline across every file in a workspace, use the workspace export instead. See [Exporting a Timeline](/docs/workspaces/exporting-a-timeline/).
