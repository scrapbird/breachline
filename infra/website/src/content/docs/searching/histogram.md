---
title: "Timeline Histogram"
date: 2025-01-01T00:00:00Z
draft: false
weight: 3
---

The timeline histogram sits above the grid and shows how many events fall into each time bucket. It is the fastest way to spot bursts of activity and to zoom into a window of interest.

## Reading the histogram

- Each bar is a time bucket; its height is the number of events in that bucket.
- The bars always reflect your current query, so filtering the grid redraws the histogram to match.
- Axis labels are rendered in your [display timezone](/docs/loading-data/timestamps-timezones/).
- Hover a bar to see its exact count and time range.

Toggle the histogram with **View → Toggle Histogram** or the keyboard shortcut (see [Keybindings](/docs/configuration/keybindings/)).

## Filter by dragging

Click and drag across the histogram to select a time range. BreachLine turns the selection into `after`/`before` [time filters](/docs/searching/query-pipeline/) and applies them to your query, so the grid narrows to exactly that window.

If your query already contains a time range, dragging a new selection replaces it rather than stacking a second one.

## Copying the histogram

You can copy the histogram as a PNG image to paste into a report or ticket. See [Copying & Exporting](/docs/searching/copying-exporting/).
