---
title: "Config System"
date: 2025-01-01T00:00:00Z
draft: false
weight: 1
---

BreachLine keeps its configuration simple: a single settings screen, sensible defaults, and one small file on disk.

## Opening settings

Choose **File → Settings** (or press the settings shortcut, see [Keybindings](/docs/configuration/keybindings/)). Changes take effect when you save, and you can cancel to discard them.

## Available settings

| Setting | What it does | Default |
|---|---|---|
| Sort by time | Order rows by the timestamp column | On |
| Sort descending | Newest rows first when sorting by time | Off (oldest first) |
| Enable query cache | Cache filtered query results for speed | On |
| Default ingest timezone | Zone applied to timestamps with no offset | Local |
| Display timezone | Zone used to display timestamps | Local |
| Timestamp display format | Text format for timestamps | `yyyy-MM-dd HH:mm:ss` |

The timezone and format settings are explained in detail in [Timestamps & Timezones](/docs/loading-data/timestamps-timezones/). The cache setting is covered in [Cache System](/docs/configuration/cache-system/).

## How changes are applied

- **Sort changes** clear the cached data so the next query re-sorts against the new order.
- **Timezone and format changes** re-render the grid without re-reading the file.
- **Cache toggle** turns query-result caching on or off immediately.

## Where settings are stored

Settings live in a small `breachline.yml` file alongside the application. Only values you have changed from the defaults are written, so the file stays minimal and any setting you have not touched follows the current default.

Your license is stored with your settings but is never shown on the settings screen. See [Licensing](/docs/getting-started/licensing/).
